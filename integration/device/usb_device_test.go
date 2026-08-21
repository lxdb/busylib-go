//go:build device

package device_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lxdb/busylib-go/usb"
)

func TestUSBDeviceReadContracts(t *testing.T) {
	address := os.Getenv("BUSYBAR_USB_ADDRESS")
	if address == "" {
		t.Skip("BUSYBAR_USB_ADDRESS is not set")
	}
	client, err := usb.NewClient(
		usb.WithAddress(address),
		usb.WithDialTimeout(5*time.Second),
		usb.WithCommandTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := client.Commands().Uptime(ctx)
	if err != nil {
		t.Fatalf("Uptime: %v", err)
	}
	if strings.TrimSpace(response.Output) == "" {
		t.Fatal("uptime command returned empty output")
	}
}
