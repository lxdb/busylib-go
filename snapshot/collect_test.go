package snapshot_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	busylib "github.com/lxdb/busylib-go"
	"github.com/lxdb/busylib-go/proto/blepb"
	"github.com/lxdb/busylib-go/proto/statepb"
	"github.com/lxdb/busylib-go/snapshot"
)

func TestCollectCanonicalFirmwareSnapshot(t *testing.T) {
	var (
		mu    sync.Mutex
		paths []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()

		if r.URL.Path != "/api/version" && r.Header.Get("X-API-Sem-Ver") != "25.0.0" {
			t.Errorf("%s X-API-Sem-Ver = %q", r.URL.Path, r.Header.Get("X-API-Sem-Ver"))
		}
		writeSnapshotResponse(t, w, r.URL.Path)
	}))
	t.Cleanup(server.Close)

	client, err := busylib.NewClient(busylib.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	got, err := snapshot.Collect(context.Background(), client)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if !got.Complete() || got.Empty() {
		t.Fatalf("complete=%v empty=%v failures=%v", got.Complete(), got.Empty(), got.Failures())
	}
	if got.Name.Value != "Desk" || got.Version.Value != "25.0.0" {
		t.Fatalf("identity = name %q version %q", got.Name.Value, got.Version.Value)
	}
	if !got.Power.Value.Known || got.Power.Value.BatteryStatus != statepb.BatteryStatus_CHARGING || got.Power.Value.BatteryCurrentMA != -125 {
		t.Fatalf("power = %#v", got.Power.Value)
	}
	if got.Brightness.Value.Mode != snapshot.BrightnessModeAutomatic || got.Brightness.Value.Manual != nil {
		t.Fatalf("brightness = %#v", got.Brightness.Value)
	}
	if got.AudioVolume.Value != 40 {
		t.Fatalf("audio volume = %d", got.AudioVolume.Value)
	}
	if got.WiFi.Value.State != snapshot.WiFiStateActive || got.WiFi.Value.ConnectionStatus == nil || *got.WiFi.Value.ConnectionStatus != statepb.WifiConnectionStatus_CONNECTED {
		t.Fatalf("wifi = %#v", got.WiFi.Value)
	}
	if len(got.WiFi.Value.IPAddresses) != 1 || got.WiFi.Value.IPAddresses[0].Address != "192.168.1.20" {
		t.Fatalf("wifi addresses = %#v", got.WiFi.Value.IPAddresses)
	}
	if got.BLE.Value.Status != blepb.ServiceStatus_CONNECTED || got.BLE.Value.Address == nil || *got.BLE.Value.Address != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("ble = %#v", got.BLE.Value)
	}
	wantTime, err := time.Parse(time.RFC3339, "2026-07-15T12:30:45-06:00")
	if err != nil {
		t.Fatal(err)
	}
	if got.Time.Value.Timestamp != "2026-07-15T12:30:45-06:00" || !got.Time.Value.Time.Equal(wantTime) {
		t.Fatalf("time = %#v", got.Time.Value)
	}

	wantPaths := []string{
		"/api/version",
		"/api/name",
		"/api/status",
		"/api/status/system",
		"/api/status/power",
		"/api/time",
		"/api/wifi/status",
		"/api/display/brightness",
		"/api/audio/volume",
		"/api/ble/status",
		"/api/storage/status",
	}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("paths = %#v, want %#v", paths, wantPaths)
	}
}

func TestCollectPreservesPartialFailuresAndRawMalformedPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/time":
			_, _ = w.Write([]byte(`{"timestamp":"not-a-time"}`))
		case "/api/wifi/status":
			http.Error(w, `{"error":"wifi unavailable"}`, http.StatusServiceUnavailable)
		default:
			writeSnapshotResponse(t, w, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	client, err := busylib.NewClient(busylib.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	got, err := snapshot.Collect(context.Background(), client)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if got.Complete() || got.Empty() || !got.Name.Present {
		t.Fatalf("complete=%v empty=%v name=%#v", got.Complete(), got.Empty(), got.Name)
	}
	var protocolError *busylib.ProtocolError
	if !errors.As(got.Time.Err, &protocolError) {
		t.Fatalf("time error = %T %v", got.Time.Err, got.Time.Err)
	}
	if string(got.Time.Raw) != `{"timestamp":"not-a-time"}` {
		t.Fatalf("time raw = %q", got.Time.Raw)
	}
	var apiError *busylib.APIError
	if !errors.As(got.WiFi.Err, &apiError) || apiError.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("wifi error = %T %v", got.WiFi.Err, got.WiFi.Err)
	}
	if failures := got.Failures(); len(failures) != 2 || failures[snapshot.SectionTime] == nil || failures[snapshot.SectionWiFi] == nil {
		t.Fatalf("failures = %#v", failures)
	}
}

func TestCollectRejectsNonCanonicalNameAliases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/name" {
			_, _ = w.Write([]byte(`{"device":"Desk","value":"Desk"}`))
			return
		}
		writeSnapshotResponse(t, w, r.URL.Path)
	}))
	t.Cleanup(server.Close)

	client, err := busylib.NewClient(busylib.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	got, err := snapshot.Collect(context.Background(), client)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if got.Name.Present || got.Name.Err == nil {
		t.Fatalf("name = %#v", got.Name)
	}
	if string(got.Name.Raw) != `{"device":"Desk","value":"Desk"}` {
		t.Fatalf("name raw = %q", got.Name.Raw)
	}
}

func TestCollectCancellationReturnsPartialSnapshot(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/version" {
			writeSnapshotResponse(t, w, r.URL.Path)
			return
		}
		close(requestStarted)
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	client, err := busylib.NewClient(busylib.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		<-requestStarted
		cancel()
	}()

	got, err := snapshot.Collect(ctx, client)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Collect error = %v, want context.Canceled", err)
	}
	if !got.Version.Present || got.Version.Value != "25.0.0" {
		t.Fatalf("partial version = %#v", got.Version)
	}
	if got.Name.Err == nil || got.Status.Present {
		t.Fatalf("partial snapshot = name %#v status %#v", got.Name, got.Status)
	}
}

func TestCollectRejectsInvalidSetup(t *testing.T) {
	if _, err := snapshot.Collect(nil, nil); err == nil {
		t.Fatal("Collect(nil, nil) returned nil error")
	}
	if _, err := snapshot.Collect(context.Background(), nil); err == nil {
		t.Fatal("Collect with nil client returned nil error")
	}
}

func writeSnapshotResponse(t *testing.T, w http.ResponseWriter, path string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	responses := map[string]string{
		"/api/version":            `{"api_semver":"25.0.0"}`,
		"/api/name":               `{"name":"Desk"}`,
		"/api/status":             `{"device":{"serial_number":"ABC","usb_mac":"00","otp_valid":true,"firmware_security":"signed"},"firmware":{"version":"1.0.0","target":1,"branch":"dev","build_date":"today","commit_hash":"abc","intercom_version":"1"},"system":{"api_semver":"25.0.0","uptime":"1m","boot_time":1,"auto_update_enabled":true},"power":{"state":"charging","battery_charge":75,"battery_voltage":3800,"battery_current":-125,"usb_voltage":5000}}`,
		"/api/status/system":      `{"api_semver":"25.0.0","uptime":"1m","boot_time":1,"auto_update_enabled":true}`,
		"/api/status/power":       `{"state":"charging","battery_charge":75,"battery_voltage":3800,"battery_current":-125,"usb_voltage":5000}`,
		"/api/time":               `{"timestamp":"2026-07-15T12:30:45-06:00"}`,
		"/api/wifi/status":        `{"state":"connected","ssid":"Desk","bssid":"11:22:33:44:55:66","channel":6,"rssi":-40,"security":"WPA2","ip_config":{"ip_method":"dhcp","ip_type":"ipv4","address":"192.168.1.20"}}`,
		"/api/display/brightness": `{"value":"auto"}`,
		"/api/audio/volume":       `{"volume":40}`,
		"/api/ble/status":         `{"status":"connected","address":"AA:BB:CC:DD:EE:FF"}`,
		"/api/storage/status":     `{"used_bytes":10,"free_bytes":20,"total_bytes":30}`,
	}
	payload, ok := responses[path]
	if !ok {
		t.Errorf("unexpected path %s", path)
		http.NotFound(w, nil)
		return
	}
	_, _ = w.Write([]byte(payload))
}
