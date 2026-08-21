//go:build device

package device_test

import (
	"context"
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
			t.Fatal("status stream closed before delivering a message")
		}
		if message.State == nil && len(message.Updates) == 0 {
			t.Fatalf("status stream delivered an empty message: %#v", message)
		}
	case err := <-statusStream.Errors():
		t.Fatalf("status stream error: %v", err)
	case <-ctx.Done():
		t.Fatalf("status stream message: %v", ctx.Err())
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
