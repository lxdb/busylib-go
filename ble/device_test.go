//go:build device && darwin && cgo

package ble_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	busylib "github.com/lxdb/busylib-go"
	"github.com/lxdb/busylib-go/ble"
	"github.com/lxdb/busylib-go/frame"
)

func TestBLEDeviceDataPlane(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	peripherals, err := ble.Scan(ctx, 5*time.Second)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	identifier := ble.Identifier(os.Getenv("BUSYBAR_BLE_IDENTIFIER"))
	if identifier == "" {
		identifier = peripherals[0].Identifier
	} else if !containsIdentifier(peripherals, identifier) {
		t.Fatalf("scan did not return selected identifier %q", identifier)
	}

	client, err := ble.Connect(ctx, identifier)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close BLE client: %v", err)
		}
	})

	version, err := client.Device().System().Version(ctx)
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if version.APISemVer == "" {
		t.Fatal("Version returned an empty API version")
	}

	statusStream, err := client.NewStatusStream()
	if err != nil {
		t.Fatalf("NewStatusStream: %v", err)
	}
	if err := statusStream.Start(ctx); err != nil {
		t.Fatalf("Start status stream: %v", err)
	}
	defer statusStream.Stop()
	select {
	case message, ok := <-statusStream.Messages():
		if !ok {
			t.Fatalf("status stream closed before a message: %v", statusStream.Wait())
		}
		if message.State == nil && len(message.Updates) == 0 {
			t.Fatalf("status stream delivered an empty message: %#v", message)
		}
	case <-ctx.Done():
		t.Fatalf("wait for status message: %v", ctx.Err())
	}

	raw, err := client.Device().Display().Screen(ctx, busylib.DisplayFront)
	if err != nil {
		t.Fatalf("Screen: %v", err)
	}
	captured, err := frame.FromHTTP(busylib.DisplayFront, raw)
	if err != nil {
		t.Fatalf("decode captured frame: %v", err)
	}
	if _, err := captured.RGBA(); err != nil {
		t.Fatalf("render captured frame: %v", err)
	}

	if os.Getenv("BUSYBAR_BLE_WRITE_TEST") != "1" {
		t.Log("set BUSYBAR_BLE_WRITE_TEST=1 to include the storage round-trip")
		return
	}
	testBLEImageRoundTrip(t, ctx, client, raw)
}

func TestBLEDeviceBondedReconnect(t *testing.T) {
	identifier := requiredBLEIdentifier(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	first, err := ble.Connect(ctx, identifier)
	if err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first connection: %v", err)
	}

	second, err := ble.Connect(ctx, identifier)
	if err != nil {
		t.Fatalf("bonded reconnect without changing pairing state: %v", err)
	}
	defer second.Close()
	if _, err := second.Device().System().Version(ctx); err != nil {
		t.Fatalf("Version after bonded reconnect: %v", err)
	}
}

func testBLEImageRoundTrip(t *testing.T, ctx context.Context, client *ble.Client, raw []byte) {
	t.Helper()
	applicationName := fmt.Sprintf("ble-%x", time.Now().UnixNano())
	const fileName = "captured-front.rgb"
	path := "/ext/user_assets/" + applicationName + "/" + fileName
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.Device().Assets().DeleteApplicationAssets(cleanupCtx, applicationName); err != nil {
			t.Errorf("cleanup uploaded BLE test image: %v", err)
		}
	})

	if err := client.Device().Assets().Upload(ctx, busylib.UploadAssetRequest{
		ApplicationName: applicationName,
		File:            fileName,
		Body:            busylib.BytesBody(raw, "application/octet-stream"),
	}); err != nil {
		t.Fatalf("upload captured image: %v", err)
	}
	downloaded, err := client.Device().Storage().Read(ctx, path)
	if err != nil {
		t.Fatalf("download captured image: %v", err)
	}
	if !bytes.Equal(downloaded, raw) {
		t.Fatalf("downloaded image differs: got %d bytes, want %d", len(downloaded), len(raw))
	}
	if _, err := frame.FromHTTP(busylib.DisplayFront, downloaded); err != nil {
		t.Fatalf("decode downloaded image: %v", err)
	}
}

func requiredBLEIdentifier(t *testing.T) ble.Identifier {
	t.Helper()
	value := os.Getenv("BUSYBAR_BLE_IDENTIFIER")
	if value == "" {
		t.Fatal("BUSYBAR_BLE_IDENTIFIER is required for physical BLE tests")
	}
	return ble.Identifier(value)
}

func containsIdentifier(peripherals []ble.Peripheral, want ble.Identifier) bool {
	for _, peripheral := range peripherals {
		if peripheral.Identifier == want {
			return true
		}
	}
	return false
}
