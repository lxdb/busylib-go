//go:build device

package device_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	busylib "github.com/lxdb/busylib-go"
	"github.com/lxdb/busylib-go/snapshot"
)

func TestLocalDeviceReadContracts(t *testing.T) {
	client := newLocalDeviceClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	version, err := client.APISemVer(ctx)
	if err != nil {
		t.Fatalf("APISemVer: %v", err)
	}
	if version == "" {
		t.Fatal("device returned an empty API version")
	}

	status, err := client.System().Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Firmware.Version == "" || status.Device.SerialNumber == "" {
		t.Fatalf("status lacks stable identity fields: firmware=%q serial_empty=%t", status.Firmware.Version, status.Device.SerialNumber == "")
	}

	state, err := snapshot.Collect(ctx, client)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if !state.Complete() {
		t.Fatalf("snapshot is incomplete: failures=%v", state.Failures())
	}
	if len(state.Failures()) != 0 {
		t.Fatalf("snapshot field failures = %v", state.Failures())
	}
}

func TestLocalDeviceStatusStreamLifecycle(t *testing.T) {
	client := newLocalDeviceClient(t)
	statusStream, err := client.NewStatusStream()
	if err != nil {
		t.Fatalf("NewStatusStream: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := statusStream.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		if err := statusStream.Stop(); err != nil {
			t.Errorf("Stop: %v", err)
		}
	}()
	if err := statusStream.RequestSnapshot(ctx); err != nil {
		t.Fatalf("RequestSnapshot: %v", err)
	}

	select {
	case message, ok := <-statusStream.Messages():
		if !ok {
			t.Fatalf("status stream closed before delivering a message: %v", statusStream.Wait())
		}
		if message.State == nil && len(message.Updates) == 0 {
			t.Fatalf("status stream delivered an empty message: %#v", message)
		}
	case <-ctx.Done():
		t.Fatalf("status stream message: %v", ctx.Err())
	}
}

func TestLocalDeviceAssetUploadReadBackAndCleanup(t *testing.T) {
	client := newLocalDeviceClient(t)
	applicationName := fmt.Sprintf("busylib-go-%x", time.Now().UnixNano())
	const fileName = "release-probe.bin"
	devicePath := "/ext/user_assets/" + applicationName + "/" + fileName
	payload := []byte("busylib-go physical media probe\n")

	cleanupNeeded := true
	t.Cleanup(func() {
		if !cleanupNeeded {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.Assets().DeleteApplicationAssets(ctx, applicationName); err != nil {
			t.Errorf("cleanup application assets: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.Assets().Upload(ctx, busylib.UploadAssetRequest{
		ApplicationName: applicationName,
		File:            fileName,
		Body:            busylib.BytesBody(payload, "application/octet-stream"),
	}); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	got, err := client.Storage().Read(ctx, devicePath)
	if err != nil {
		t.Fatalf("Read uploaded asset: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("uploaded asset = %q, want %q", got, payload)
	}

	if err := client.Assets().DeleteApplicationAssets(ctx, applicationName); err != nil {
		t.Fatalf("DeleteApplicationAssets: %v", err)
	}
	cleanupNeeded = false

	assets, err := client.Storage().List(ctx, "/ext/user_assets")
	if err != nil {
		t.Fatalf("List user assets after cleanup: %v", err)
	}
	for _, asset := range assets.List {
		if asset.Name == applicationName {
			t.Fatalf("application asset directory %q remains after cleanup", applicationName)
		}
	}
}

func newLocalDeviceClient(t *testing.T) *busylib.Client {
	t.Helper()
	baseURL := os.Getenv("BUSYBAR_BASE_URL")
	if baseURL == "" {
		t.Skip("BUSYBAR_BASE_URL is not set")
	}
	options := []busylib.Option{
		busylib.WithBaseURL(baseURL),
		busylib.WithTimeout(10 * time.Second),
	}
	if key := os.Getenv("BUSYBAR_ACCESS_KEY"); key != "" {
		options = append(options, busylib.WithLocalAccessKey(key))
	}
	client, err := busylib.NewClient(options...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}
