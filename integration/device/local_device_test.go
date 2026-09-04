//go:build device

package device_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/color"
	"net/http"
	"os"
	"testing"
	"time"

	busylib "github.com/lxdb/busylib-go"
	"github.com/lxdb/busylib-go/frame"
	"github.com/lxdb/busylib-go/snapshot"
)

func TestLocalDeviceReadContracts(t *testing.T) {
	client := newLocalDeviceClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	status := requireExpectedDevice(t, ctx, client)
	if status.Device.SerialNumber == "" {
		t.Fatal("status has an empty device serial number")
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

func TestLocalDeviceAccessTokenLifecycle(t *testing.T) {
	admin := newLocalDeviceClient(t)
	name := fmt.Sprintf("busylib-token-%x", time.Now().UnixNano())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	requireExpectedDevice(t, ctx, admin)
	minted, err := admin.Settings().MintAccessToken(ctx, name)
	if err != nil {
		t.Fatalf("MintAccessToken: %v", err)
	}
	if minted.ShortID == "" || minted.Token == "" {
		t.Fatalf("minted token lacks identifiers: short_id_empty=%t token_empty=%t", minted.ShortID == "", minted.Token == "")
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		err := admin.Settings().RevokeAccessToken(cleanupCtx, minted.ShortID)
		var apiErr *busylib.APIError
		if err != nil && (!errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound) {
			t.Errorf("cleanup access token: %v", err)
		}
	})

	info, err := admin.Settings().AccessTokens(ctx)
	if err != nil {
		t.Fatalf("AccessTokens: %v", err)
	}
	if !containsAccessToken(info.Tokens, minted.ShortID, name) {
		t.Fatalf("access token list does not contain short ID %q and name %q", minted.ShortID, name)
	}

	tokenClient := newLocalDeviceClientWithToken(t, minted.Token)
	if _, err := tokenClient.System().Status(ctx); err != nil {
		t.Fatalf("token-authenticated Status: %v", err)
	}
	if _, err := tokenClient.Settings().MintAccessToken(ctx, name); err == nil {
		t.Fatal("token-authenticated MintAccessToken succeeded, want authorization failure")
	} else {
		var apiErr *busylib.APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
			t.Fatalf("token-authenticated MintAccessToken error = %v, want HTTP 403", err)
		}
	}
	if err := tokenClient.Settings().RevokeAccessToken(ctx, minted.ShortID); err != nil {
		t.Fatalf("self RevokeAccessToken: %v", err)
	}
	if _, err := tokenClient.System().Status(ctx); err == nil {
		t.Fatal("revoked token still authenticates")
	} else {
		var apiErr *busylib.APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
			t.Fatalf("revoked token Status error = %v, want HTTP 403", err)
		}
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

func TestLocalDeviceAssetAndStorageWriteLifecycle(t *testing.T) {
	client := newLocalDeviceClient(t)
	applicationName := fmt.Sprintf("busylib-%x", time.Now().UnixNano())
	const fileName = "nested/probe.bin"
	devicePath := "/ext/user_assets/" + applicationName + "/" + fileName
	payload := []byte("busylib-go physical media probe\n")
	appendPath := "/ext/user_assets/" + applicationName + "/append-probe.bin"

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
	requireExpectedDevice(t, ctx, client)
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

	appendEnabled := true
	if err := client.Storage().Write(ctx, busylib.WriteStorageFileRequest{
		Path: appendPath,
		Body: busylib.BytesBody([]byte("A"), "application/octet-stream"),
	}); err != nil {
		t.Fatalf("Write replacement: %v", err)
	}
	if err := client.Storage().Write(ctx, busylib.WriteStorageFileRequest{
		Path:   appendPath,
		Body:   busylib.BytesBody([]byte("B"), "application/octet-stream"),
		Append: &appendEnabled,
	}); err != nil {
		t.Fatalf("Write append: %v", err)
	}
	appended, err := client.Storage().Read(ctx, appendPath)
	if err != nil {
		t.Fatalf("Read appended file: %v", err)
	}
	if string(appended) != "AB" {
		t.Fatalf("appended file = %q, want %q", appended, "AB")
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

// TestLocalDeviceSelectiveClear runs last because firmware 1.2.3 can restart
// while processing its unterminated internal element_ids pointer array.
func TestLocalDeviceSelectiveClear(t *testing.T) {
	client := newLocalDeviceClient(t)
	applicationName := fmt.Sprintf("busylib-%x", time.Now().UnixNano())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	before := requireExpectedDevice(t, ctx, client)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := client.Display().Clear(cleanupCtx, applicationName); err != nil {
			t.Errorf("cleanup display application: %v", err)
		}
	})

	borderWidth, backgroundZ, bitmapZ := 0, 10, 20
	background := busylib.NewRectangleElement("background", 4, 4)
	background.Fill = busylib.RectangleFillSolid
	background.FillColors = []string{"#FF0000FF"}
	background.BorderWidth = &borderWidth
	background.ZIndex = &backgroundZ
	bitmap := busylib.NewXPMBitmapElement("bitmap", "! XPM2\n4 4 1 1\n+ c #FFFFFF\n++++\n++++\n++++\n++++")
	bitmap.ZIndex = &bitmapZ
	draw := busylib.NewDisplayElements(applicationName, background, bitmap)
	draw.Priority = 100
	if err := client.Display().Draw(ctx, draw); err != nil {
		t.Fatalf("Draw layered XPM bitmap: %v", err)
	}
	waitForFrontPixel(t, ctx, client, color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})

	if err := client.Display().ClearElements(ctx, busylib.ClearDisplayElementsRequest{
		ApplicationName: applicationName,
		ElementIDs:      []string{"bitmap"},
	}); err != nil {
		t.Fatalf("ClearElements: %v", err)
	}
	waitForFrontPixel(t, ctx, client, color.RGBA{R: 0xff, A: 0xff})

	after, err := client.System().Status(ctx)
	if err != nil {
		t.Fatalf("Status after selective clear: %v", err)
	}
	if after.System.BootTime != before.System.BootTime {
		t.Fatalf("device restarted during selective clear: boot_time before=%d after=%d", before.System.BootTime, after.System.BootTime)
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
		options = append(options, busylib.WithLocalAccessToken(key))
	}
	client, err := busylib.NewClient(options...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func expectedDeviceVersions(t *testing.T) (firmware, api string) {
	t.Helper()
	firmware = os.Getenv("BUSYBAR_EXPECTED_FIRMWARE_VERSION")
	if firmware == "" {
		t.Skip("BUSYBAR_EXPECTED_FIRMWARE_VERSION is not set")
	}
	api = os.Getenv("BUSYBAR_EXPECTED_API_VERSION")
	if api == "" {
		t.Skip("BUSYBAR_EXPECTED_API_VERSION is not set")
	}
	return firmware, api
}

func requireExpectedDevice(t *testing.T, ctx context.Context, client *busylib.Client) busylib.Status {
	t.Helper()
	expectedFirmware, expectedAPI := expectedDeviceVersions(t)
	version, err := client.APISemVer(ctx)
	if err != nil {
		t.Fatalf("APISemVer: %v", err)
	}
	if version != expectedAPI {
		t.Fatalf("API version = %q, want %q", version, expectedAPI)
	}
	status, err := client.System().Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Firmware.Version != expectedFirmware {
		t.Fatalf("firmware version = %q, want %q", status.Firmware.Version, expectedFirmware)
	}
	if status.System.APISemVer != expectedAPI {
		t.Fatalf("status API version = %q, want %q", status.System.APISemVer, expectedAPI)
	}
	return status
}

func newLocalDeviceClientWithToken(t *testing.T, token string) *busylib.Client {
	t.Helper()
	client, err := busylib.NewClient(
		busylib.WithBaseURL(os.Getenv("BUSYBAR_BASE_URL")),
		busylib.WithTimeout(10*time.Second),
		busylib.WithLocalAccessToken(token),
	)
	if err != nil {
		t.Fatalf("NewClient with access token: %v", err)
	}
	return client
}

func containsAccessToken(tokens []busylib.StoredAccessToken, shortID, name string) bool {
	for _, token := range tokens {
		if token.ShortID == shortID && token.Name == name {
			return true
		}
	}
	return false
}

func waitForFrontPixel(t *testing.T, ctx context.Context, client *busylib.Client, want color.RGBA) {
	t.Helper()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	var lastPixel color.RGBA
	attempts := 0
	for {
		attempts++
		raw, err := client.Display().Screen(ctx, busylib.DisplayFront)
		if err != nil {
			lastErr = fmt.Errorf("screen: %w", err)
		} else {
			value, frameErr := frame.FromHTTP(busylib.DisplayFront, raw)
			if frameErr != nil {
				lastErr = fmt.Errorf("decode HTTP frame: %w", frameErr)
			} else {
				frameImage, rgbaErr := value.RGBA()
				if rgbaErr != nil {
					lastErr = fmt.Errorf("convert frame to RGBA: %w", rgbaErr)
				} else {
					lastPixel = color.RGBAModel.Convert(frameImage.At(0, 0)).(color.RGBA)
					lastErr = nil
				}
				if lastErr == nil && lastPixel == want {
					return
				}
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("front pixel did not become %#v after %d attempts: last_pixel=%#v last_error=%v: %v", want, attempts, lastPixel, lastErr, ctx.Err())
		case <-ticker.C:
		}
	}
}
