package busylib

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	internalapi "github.com/lxdb/busylib-go/internal/api"
)

func TestHTTPServiceOperationCoverageMatchesOpenAPIInventory(t *testing.T) {
	inventory, err := internalapi.BuildInventoryFile("internal/api/testdata/busybar-f21-openapi-1.0.0-rc.yaml")
	if err != nil {
		t.Fatalf("read openapi inventory: %v", err)
	}

	covered := map[string]string{
		"GET /api/status/ws": "stream phase deferral",
	}
	for _, tc := range serviceRequestCases(t) {
		operationID := tc.operationID()
		if operationID == "" {
			t.Fatalf("%s does not declare an OpenAPI operation ID", tc.name)
		}
		if existing, ok := covered[operationID]; ok {
			t.Fatalf("operation %s is covered by both %s and %s", operationID, existing, tc.name)
		}
		covered[operationID] = tc.name
	}

	inventoryIDs := make(map[string]struct{}, inventory.OperationCount)
	for _, operation := range inventory.Operations {
		inventoryIDs[operation.ID] = struct{}{}
		if _, ok := covered[operation.ID]; !ok {
			t.Fatalf("operation %s is not covered by a service method or explicit deferral", operation.ID)
		}
	}
	for operationID := range covered {
		if _, ok := inventoryIDs[operationID]; !ok {
			t.Fatalf("operation %s is covered but is not in the OpenAPI inventory", operationID)
		}
	}
	if len(covered) != inventory.OperationCount {
		t.Fatalf("covered operation count = %d, want %d", len(covered), inventory.OperationCount)
	}
	if got := covered["GET /api/status/ws"]; got != "stream phase deferral" {
		t.Fatalf("GET /api/status/ws coverage = %q, want stream phase deferral", got)
	}
}

func serviceRequestCases(t *testing.T) []serviceRequestCase {
	t.Helper()
	okJSON := `{"result":"OK"}`
	return []serviceRequestCase{
		{
			name: "system version",
			call: func(ctx context.Context, client *Client) error {
				got, err := client.System().Version(ctx)
				if err != nil {
					return err
				}
				if got.APISemVer != "24.3.0" {
					t.Fatalf("APISemVer = %q", got.APISemVer)
				}
				return nil
			},
			method:   http.MethodGet,
			path:     "/api/version",
			response: `{"api_semver":"24.3.0"}`,
		},
		jsonGetCase("system status", "/api/status", func(ctx context.Context, client *Client) error {
			got, err := client.System().Status(ctx)
			if err != nil {
				return err
			}
			if got.System.APISemVer != "24.3.0" {
				t.Fatalf("system api_semver = %q", got.System.APISemVer)
			}
			return nil
		}, `{"system":{"api_semver":"24.3.0","uptime":"00d","boot_time":1,"auto_update_enabled":true}}`),
		jsonGetCase("system device status", "/api/status/device", func(ctx context.Context, client *Client) error {
			_, err := client.System().DeviceStatus(ctx)
			return err
		}, `{"serial_number":"203","usb_mac":"00:11","otp_valid":true,"firmware_security":"secure"}`),
		jsonGetCase("system firmware status", "/api/status/firmware", func(ctx context.Context, client *Client) error {
			_, err := client.System().FirmwareStatus(ctx)
			return err
		}, `{"version":"1.0.0","target":22,"branch":"main","build_date":"2026-01-01","commit_hash":"abc"}`),
		jsonGetCase("system system status", "/api/status/system", func(ctx context.Context, client *Client) error {
			_, err := client.System().SystemStatus(ctx)
			return err
		}, `{"api_semver":"24.3.0","uptime":"00d","boot_time":1,"auto_update_enabled":true}`),
		jsonGetCase("system power status", "/api/status/power", func(ctx context.Context, client *Client) error {
			_, err := client.System().PowerStatus(ctx)
			return err
		}, `{"state":"charged","battery_charge":99,"battery_voltage":4183,"battery_current":-1,"usb_voltage":4843}`),
		jsonGetCase("system transport", "/api/transport", func(ctx context.Context, client *Client) error {
			_, err := client.System().Transport(ctx)
			return err
		}, `{"type":"usb"}`),
		{
			name:     "system dump log",
			call:     func(ctx context.Context, client *Client) error { return client.System().DumpLog(ctx, "/ext/dump.log") },
			method:   http.MethodPost,
			path:     "/api/log_dump",
			query:    "path=%2Fext%2Fdump.log",
			response: "",
		},
		jsonGetCase("settings access", "/api/access", func(ctx context.Context, client *Client) error {
			_, err := client.Settings().HTTPAccess(ctx)
			return err
		}, `{"mode":"key","key_valid":true}`),
		successCase("settings set access", http.MethodPost, "/api/access", "key=1234&mode=key", func(ctx context.Context, client *Client) error {
			return client.Settings().SetHTTPAccess(ctx, HTTPAccessKey, "1234")
		}, okJSON),
		jsonGetCase("settings name", "/api/name", func(ctx context.Context, client *Client) error {
			_, err := client.Settings().Name(ctx)
			return err
		}, `{"name":"BUSY bar"}`),
		successJSONCase("settings set name", http.MethodPost, "/api/name", "", `{"name":"Desk"}`, func(ctx context.Context, client *Client) error {
			return client.Settings().SetName(ctx, "Desk")
		}, okJSON),
		jsonGetCase("display brightness", "/api/display/brightness", func(ctx context.Context, client *Client) error {
			_, err := client.Display().Brightness(ctx)
			return err
		}, `{"value":"auto"}`),
		successCase("display set brightness", http.MethodPost, "/api/display/brightness", "value=50", func(ctx context.Context, client *Client) error {
			return client.Display().SetBrightness(ctx, "50")
		}, okJSON),
		successJSONCase("display draw", http.MethodPost, "/api/display/draw", "", `{"application_name":"app","elements":[{"id":"text","type":"text","text":"Hi","font":"normal"}]}`, func(ctx context.Context, client *Client) error {
			return client.Display().Draw(ctx, DisplayElements{
				ApplicationName: "app",
				Elements: []DisplayElement{
					TextElement{BaseDisplayElement: BaseDisplayElement{ID: "text"}, Text: "Hi", Font: FontNormal},
				},
			})
		}, okJSON),
		successCase("display clear app", http.MethodDelete, "/api/display/draw", "application_name=app", func(ctx context.Context, client *Client) error {
			return client.Display().Clear(ctx, "app")
		}, okJSON),
		{
			name: "display screen bytes",
			call: func(ctx context.Context, client *Client) error {
				got, err := client.Display().Screen(ctx, 1)
				if err != nil {
					return err
				}
				if !bytes.Equal(got, []byte{1, 2, 3}) {
					t.Fatalf("screen bytes = %v", got)
				}
				return nil
			},
			method:   http.MethodGet,
			path:     "/api/screen",
			query:    "display=1",
			response: "\x01\x02\x03",
		},
		successJSONCase("audio play", http.MethodPost, "/api/audio/play", "", `{"application_name":"app","path":"tone.snd"}`, func(ctx context.Context, client *Client) error {
			return client.Audio().Play(ctx, PlayAudio{ApplicationName: "app", Path: "tone.snd"})
		}, okJSON),
		successCase("audio stop", http.MethodDelete, "/api/audio/play", "", func(ctx context.Context, client *Client) error {
			return client.Audio().Stop(ctx)
		}, okJSON),
		jsonGetCase("audio volume", "/api/audio/volume", func(ctx context.Context, client *Client) error {
			_, err := client.Audio().Volume(ctx)
			return err
		}, `{"volume":42}`),
		successCase("audio set volume", http.MethodPost, "/api/audio/volume", "silent=1&volume=42", func(ctx context.Context, client *Client) error {
			return client.Audio().SetVolume(ctx, SetAudioVolumeRequest{Volume: 42, Silent: true})
		}, okJSON),
		successCase("asset upload", http.MethodPost, "/api/assets/upload", "application_name=app&file=data.png", func(ctx context.Context, client *Client) error {
			return client.Assets().Upload(ctx, UploadAssetRequest{ApplicationName: "app", File: "data.png", Body: BytesBody([]byte("png"), "application/octet-stream")})
		}, okJSON).withBody("png", "application/octet-stream"),
		successCase("asset delete app", http.MethodDelete, "/api/assets/upload", "application_name=app", func(ctx context.Context, client *Client) error {
			return client.Assets().DeleteApplicationAssets(ctx, "app")
		}, okJSON),
		successCase("storage write", http.MethodPost, "/api/storage/write", "path=%2Fext%2Ftest.txt", func(ctx context.Context, client *Client) error {
			return client.Storage().Write(ctx, WriteStorageFileRequest{Path: "/ext/test.txt", Body: BytesBody([]byte("payload"), "application/octet-stream")})
		}, okJSON).withBody("payload", "application/octet-stream"),
		{
			name: "storage read",
			call: func(ctx context.Context, client *Client) error {
				got, err := client.Storage().Read(ctx, "/ext/test.txt")
				if err != nil {
					return err
				}
				if string(got) != "payload" {
					t.Fatalf("storage read = %q", got)
				}
				return nil
			},
			method:   http.MethodGet,
			path:     "/api/storage/read",
			query:    "path=%2Fext%2Ftest.txt",
			response: "payload",
		},
		jsonGetQueryCase("storage list", "/api/storage/list", "path=%2Fext", func(ctx context.Context, client *Client) error {
			_, err := client.Storage().List(ctx, "/ext")
			return err
		}, `{"list":[{"type":"file","name":"test.txt","size":7}]}`),
		successCase("storage remove", http.MethodDelete, "/api/storage/remove", "path=%2Fext%2Ftest.txt", func(ctx context.Context, client *Client) error {
			return client.Storage().Remove(ctx, "/ext/test.txt")
		}, okJSON),
		successCase("storage mkdir", http.MethodPost, "/api/storage/mkdir", "path=%2Fext%2Fdir", func(ctx context.Context, client *Client) error {
			return client.Storage().Mkdir(ctx, "/ext/dir")
		}, okJSON),
		successCase("storage rename", http.MethodPost, "/api/storage/rename", "new_path=%2Fext%2Fnew.txt&path=%2Fext%2Fold.txt", func(ctx context.Context, client *Client) error {
			return client.Storage().Rename(ctx, "/ext/old.txt", "/ext/new.txt")
		}, okJSON),
		jsonGetCase("storage status", "/api/storage/status", func(ctx context.Context, client *Client) error {
			_, err := client.Storage().Status(ctx)
			return err
		}, `{"used_bytes":1,"free_bytes":2,"total_bytes":3}`),
		jsonGetCase("busy snapshot", "/api/busy/snapshot", func(ctx context.Context, client *Client) error {
			_, err := client.Busy().Snapshot(ctx)
			return err
		}, `{"snapshot":{"type":"NOT_STARTED","busy_bar_settings":{"theme":"on_air","show_work_phase_only":false,"trigger_smart_home":true}},"snapshot_timestamp_ms":1}`),
		successJSONCase("busy set snapshot", http.MethodPut, "/api/busy/snapshot", "", `{"snapshot":{"type":"NOT_STARTED","busy_bar_settings":{"theme":"on_air","show_work_phase_only":false,"trigger_smart_home":true}},"snapshot_timestamp_ms":1}`, func(ctx context.Context, client *Client) error {
			return client.Busy().SetSnapshot(ctx, BusySnapshot{Snapshot: BusySnapshotData{Type: BusySnapshotNotStarted, BusyBarSettings: BusyBarSettings{Theme: "on_air", TriggerSmartHome: true}}, SnapshotTimestampMS: 1})
		}, okJSON),
		jsonGetCase("busy profile", "/api/busy/profiles/busy", func(ctx context.Context, client *Client) error {
			_, err := client.Busy().Profile(ctx, BusyProfileSlotBusy)
			return err
		}, `{"sort_order":1,"title":"study","id":"00000000-0000-0000-0000-000000000000","timer_settings":{"type":"INFINITE"},"busy_bar_settings":{"theme":"on_air","show_work_phase_only":false,"trigger_smart_home":true},"profile_timestamp_ms":1}`).withOperationID("GET /api/busy/profiles/{slot}"),
		successJSONCase("busy set profile", http.MethodPut, "/api/busy/profiles/custom", "", `{"sort_order":1,"title":"study","id":"00000000-0000-0000-0000-000000000000","timer_settings":{"type":"INFINITE"},"busy_bar_settings":{"theme":"on_air","show_work_phase_only":false,"trigger_smart_home":true},"profile_timestamp_ms":1}`, func(ctx context.Context, client *Client) error {
			return client.Busy().SetProfile(ctx, BusyProfileSlotCustom, BusyProfile{SortOrder: 1, Title: "study", ID: "00000000-0000-0000-0000-000000000000", TimerSettings: BusyTimerSettings{Type: BusyTimerInfinite}, BusyBarSettings: BusyBarSettings{Theme: "on_air", TriggerSmartHome: true}, ProfileTimestampMS: 1})
		}, okJSON).withOperationID("PUT /api/busy/profiles/{slot}"),
		jsonGetCase("account status", "/api/account/status", func(ctx context.Context, client *Client) error { _, err := client.Account().Status(ctx); return err }, `{"status":"connected"}`),
		jsonGetCase("account info", "/api/account/info", func(ctx context.Context, client *Client) error { _, err := client.Account().Info(ctx); return err }, `{"linked":true,"id":"id","email":"name@example.com","user_id":"user"}`),
		jsonPostCase("account link", "/api/account/link", "", func(ctx context.Context, client *Client) error { _, err := client.Account().Link(ctx); return err }, `{"code":"ABCD","expires_at":1}`),
		jsonGetCase("account backend", "/api/account/backend", func(ctx context.Context, client *Client) error { _, err := client.Account().Backend(ctx); return err }, `{"server_url":"default","client_cert_type":"default","ignore_server_cert":false}`),
		successJSONCase("account set backend", http.MethodPut, "/api/account/backend", "", `{"server_url":"default","client_cert_type":"default","ignore_server_cert":false}`, func(ctx context.Context, client *Client) error {
			return client.Account().SetBackend(ctx, AccountBackend{ServerURL: "default", ClientCertType: AccountClientCertDefault})
		}, okJSON),
		successCase("account unlink", http.MethodDelete, "/api/account", "", func(ctx context.Context, client *Client) error { return client.Account().Unlink(ctx) }, okJSON),
		jsonGetCase("ble status", "/api/ble/status", func(ctx context.Context, client *Client) error { _, err := client.BLE().Status(ctx); return err }, `{"status":"connected","address":"50:DA:D6:FE:DD:A9"}`),
		successCase("ble enable", http.MethodPost, "/api/ble/enable", "", func(ctx context.Context, client *Client) error { return client.BLE().Enable(ctx) }, okJSON),
		successCase("ble disable", http.MethodPost, "/api/ble/disable", "", func(ctx context.Context, client *Client) error { return client.BLE().Disable(ctx) }, okJSON),
		successCase("ble remove pairing", http.MethodDelete, "/api/ble/pairing", "", func(ctx context.Context, client *Client) error { return client.BLE().RemovePairing(ctx) }, okJSON),
		jsonGetCase("wifi status", "/api/wifi/status", func(ctx context.Context, client *Client) error { _, err := client.WiFi().Status(ctx); return err }, `{"state":"connected","ssid":"ssid","security":"WPA3"}`),
		jsonGetCase("wifi networks", "/api/wifi/networks", func(ctx context.Context, client *Client) error { _, err := client.WiFi().Networks(ctx); return err }, `{"count":1,"networks":[{"ssid":"ssid","security":"WPA3","rssi":-58}]}`),
		successJSONCase("wifi connect", http.MethodPost, "/api/wifi/connect", "", `{"ssid":"ssid","password":"pass","security":"WPA3","ip_config":{"ip_method":"dhcp"}}`, func(ctx context.Context, client *Client) error {
			return client.WiFi().Connect(ctx, ConnectRequestConfig{SSID: "ssid", Password: "pass", Security: WiFiSecurityWPA3, IPConfig: &WiFiConnectIPConfig{IPMethod: WiFiIPMethodDHCP}})
		}, okJSON),
		successCase("wifi disconnect", http.MethodPost, "/api/wifi/disconnect", "", func(ctx context.Context, client *Client) error { return client.WiFi().Disconnect(ctx) }, okJSON),
		successCase("input key", http.MethodPost, "/api/input", "key=ok", func(ctx context.Context, client *Client) error { return client.Input().SendKey(ctx, InputKeyOK) }, okJSON),
		jsonGetCase("smart home pairing status", "/api/smart_home/pairing", func(ctx context.Context, client *Client) error {
			_, err := client.SmartHome().PairingStatus(ctx)
			return err
		}, `{"fabric_count":1,"latest_pairing_status":{"value":"started","timestamp":1}}`),
		jsonPostCase("smart home start pairing", "/api/smart_home/pairing", "", func(ctx context.Context, client *Client) error {
			_, err := client.SmartHome().StartPairing(ctx)
			return err
		}, `{"available_until":"1769437579000","qr_code":"MT:","manual_code":"1155"}`),
		successCase("smart home forget pairings", http.MethodDelete, "/api/smart_home/pairing", "", func(ctx context.Context, client *Client) error { return client.SmartHome().ForgetPairings(ctx) }, okJSON),
		jsonGetCase("smart home switch", "/api/smart_home/switch", func(ctx context.Context, client *Client) error {
			_, err := client.SmartHome().SwitchState(ctx)
			return err
		}, `{"state":false}`),
		successJSONCase("smart home set switch", http.MethodPost, "/api/smart_home/switch", "", `{"state":false,"startup":"last"}`, func(ctx context.Context, client *Client) error {
			return client.SmartHome().SetSwitchState(ctx, SmartHomeSwitchState{State: false, Startup: SmartHomeSwitchStartupLast})
		}, okJSON),
		jsonGetCase("time now", "/api/time", func(ctx context.Context, client *Client) error { _, err := client.Time().Now(ctx); return err }, `{"timestamp":"2025-10-02T14:30:45+04:00"}`),
		successCase("time set timestamp", http.MethodPost, "/api/time/timestamp", "timestamp=2025-10-02T14%3A30%3A45Z", func(ctx context.Context, client *Client) error {
			return client.Time().SetTimestamp(ctx, "2025-10-02T14:30:45Z")
		}, okJSON),
		jsonGetCase("time timezone", "/api/time/timezone", func(ctx context.Context, client *Client) error { _, err := client.Time().Timezone(ctx); return err }, `{"name":"Berlin","offset":"+01:00","abbr":"CET"}`),
		successCase("time set timezone", http.MethodPost, "/api/time/timezone", "timezone=Berlin", func(ctx context.Context, client *Client) error { return client.Time().SetTimezone(ctx, "Berlin") }, okJSON),
		jsonGetCase("time zones", "/api/time/tzlist", func(ctx context.Context, client *Client) error { _, err := client.Time().Timezones(ctx); return err }, `{"list":[{"name":"Berlin","offset":"+01:00","abbr":"CET"}]}`),
		successCase("update upload package", http.MethodPost, "/api/update", "", func(ctx context.Context, client *Client) error {
			return client.Update().UploadPackage(ctx, BytesBody([]byte("tar"), "application/octet-stream"))
		}, okJSON).withBody("tar", "application/octet-stream"),
		successCase("update check", http.MethodPost, "/api/update/check", "", func(ctx context.Context, client *Client) error { return client.Update().Check(ctx) }, okJSON),
		jsonGetCase("update status", "/api/update/status", func(ctx context.Context, client *Client) error { _, err := client.Update().Status(ctx); return err }, `{"install":{"is_allowed":true,"event":"none","action":"none","status":"ok"},"check":{"available_version":"1.2.3","event":"stop","status":"available"}}`),
		jsonGetQueryCase("update changelog", "/api/update/changelog", "version=1.2.3", func(ctx context.Context, client *Client) error {
			_, err := client.Update().Changelog(ctx, "1.2.3")
			return err
		}, `{"changelog":"fixed things"}`),
		successCase("update install", http.MethodPost, "/api/update/install", "version=1.2.3", func(ctx context.Context, client *Client) error { return client.Update().Install(ctx, "1.2.3") }, okJSON),
		successCase("update abort download", http.MethodPost, "/api/update/abort_download", "", func(ctx context.Context, client *Client) error { return client.Update().AbortDownload(ctx) }, okJSON),
		jsonGetCase("update autoupdate", "/api/update/autoupdate", func(ctx context.Context, client *Client) error { _, err := client.Update().Autoupdate(ctx); return err }, `{"is_enabled":true,"interval_start":"00:00","interval_end":"08:00"}`),
		successJSONCase("update set autoupdate", http.MethodPost, "/api/update/autoupdate", "", `{"is_enabled":true,"interval_start":"00:00","interval_end":"08:00"}`, func(ctx context.Context, client *Client) error {
			enabled := true
			return client.Update().SetAutoupdate(ctx, AutoupdateSettings{IsEnabled: &enabled, IntervalStart: "00:00", IntervalEnd: "08:00"})
		}, okJSON),
	}
}

func TestHTTPServicesSendExpectedRequests(t *testing.T) {
	ctx := context.Background()
	for _, tc := range serviceRequestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assertServiceRequest(t, tc, r)
				if tc.responseContentType != "" {
					w.Header().Set("Content-Type", tc.responseContentType)
				} else {
					w.Header().Set("Content-Type", "application/json")
				}
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()

			client, err := NewClient(
				WithBaseURL(server.URL),
				WithRequestIDGenerator(fixedRequestID("rid-service")),
			)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			client.setCachedAPISemVerForTest("24.3.0")

			if err := tc.call(ctx, client); err != nil {
				t.Fatalf("service call: %v", err)
			}
		})
	}
}

func TestDisplayElementsMarshalPreservesExplicitZeroValues(t *testing.T) {
	zero := 0
	payload := DisplayElements{
		ApplicationName: "app",
		Elements: []DisplayElement{
			ImageElement{BaseDisplayElement: BaseDisplayElement{ID: "image"}, Path: "image.png", Opacity: &zero},
			AnimationElement{BaseDisplayElement: BaseDisplayElement{ID: "animation"}, Path: "animation.gif", Opacity: &zero},
			RectangleElement{BaseDisplayElement: BaseDisplayElement{ID: "rectangle"}, Width: 4, Height: 2, BorderWidth: &zero},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal display elements: %v", err)
	}

	assertJSONEqual(t, string(data), `{"application_name":"app","elements":[{"id":"image","type":"image","path":"image.png","opacity":0},{"id":"animation","type":"animation","path":"animation.gif","opacity":0},{"id":"rectangle","type":"rectangle","width":4,"height":2,"border_width":0}]}`)
}

func TestWiFiConnectConfigDoesNotMarshalStatusOnlyIPType(t *testing.T) {
	payload := ConnectRequestConfig{
		SSID: "ssid",
		IPConfig: &WiFiConnectIPConfig{
			IPMethod: WiFiIPMethodStatic,
			Address:  "192.0.2.10",
			Mask:     "255.255.255.0",
			Gateway:  "192.0.2.1",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal connect request: %v", err)
	}
	if bytes.Contains(data, []byte("ip_type")) {
		t.Fatalf("connect request included status-only ip_type: %s", data)
	}

	assertJSONEqual(t, string(data), `{"ssid":"ssid","ip_config":{"ip_method":"static","address":"192.0.2.10","mask":"255.255.255.0","gateway":"192.0.2.1"}}`)
}

func TestLocalOnlyServiceMethodsAreRejectedInProxyMode(t *testing.T) {
	client, err := NewClient(
		WithEndpointMode(EndpointProxy),
		WithBaseURL("https://api.busy.app"),
		WithCloudBearerToken("cloud-token"),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("network should not be called for local-only operation")
			return nil, nil
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx := context.Background()
	checks := []struct {
		name string
		call func() error
	}{
		{"account unlink", func() error { return client.Account().Unlink(ctx) }},
		{"account set backend", func() error {
			return client.Account().SetBackend(ctx, AccountBackend{ServerURL: "default", ClientCertType: AccountClientCertDefault})
		}},
		{"account link", func() error { _, err := client.Account().Link(ctx); return err }},
		{"wifi connect", func() error { return client.WiFi().Connect(ctx, ConnectRequestConfig{SSID: "ssid"}) }},
		{"wifi disconnect", func() error { return client.WiFi().Disconnect(ctx) }},
		{"wifi networks", func() error { _, err := client.WiFi().Networks(ctx); return err }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.call(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

type serviceRequestCase struct {
	name                string
	call                func(context.Context, *Client) error
	method              string
	path                string
	query               string
	expectedBody        string
	expectedJSONBody    string
	expectedContentType string
	response            string
	responseContentType string
	openAPIOperationID  string
}

func (tc serviceRequestCase) withBody(body, contentType string) serviceRequestCase {
	tc.expectedBody = body
	tc.expectedContentType = contentType
	return tc
}

func (tc serviceRequestCase) withOperationID(operationID string) serviceRequestCase {
	tc.openAPIOperationID = operationID
	return tc
}

func (tc serviceRequestCase) operationID() string {
	if tc.openAPIOperationID != "" {
		return tc.openAPIOperationID
	}
	return tc.method + " " + tc.path
}

func jsonGetCase(name, path string, call func(context.Context, *Client) error, response string) serviceRequestCase {
	return jsonGetQueryCase(name, path, "", call, response)
}

func jsonGetQueryCase(name, path, query string, call func(context.Context, *Client) error, response string) serviceRequestCase {
	return serviceRequestCase{name: name, call: call, method: http.MethodGet, path: path, query: query, response: response}
}

func jsonPostCase(name, path, query string, call func(context.Context, *Client) error, response string) serviceRequestCase {
	return serviceRequestCase{name: name, call: call, method: http.MethodPost, path: path, query: query, response: response}
}

func successCase(name, method, path, query string, call func(context.Context, *Client) error, response string) serviceRequestCase {
	return serviceRequestCase{name: name, call: call, method: method, path: path, query: query, response: response}
}

func successJSONCase(name, method, path, query, expectedJSONBody string, call func(context.Context, *Client) error, response string) serviceRequestCase {
	return serviceRequestCase{name: name, call: call, method: method, path: path, query: query, expectedJSONBody: expectedJSONBody, response: response, expectedContentType: "application/json; charset=utf-8"}
}

func assertServiceRequest(t *testing.T, tc serviceRequestCase, r *http.Request) {
	t.Helper()
	if r.Method != tc.method {
		t.Fatalf("method = %q, want %q", r.Method, tc.method)
	}
	if r.URL.Path != tc.path {
		t.Fatalf("path = %q, want %q", r.URL.Path, tc.path)
	}
	if r.URL.RawQuery != tc.query {
		t.Fatalf("query = %q, want %q", r.URL.RawQuery, tc.query)
	}
	if got := r.Header.Get("X-API-Sem-Ver"); got != "24.3.0" && tc.path != "/api/version" {
		t.Fatalf("X-API-Sem-Ver = %q, want 24.3.0", got)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if tc.expectedContentType != "" {
		if got := r.Header.Get("Content-Type"); got != tc.expectedContentType {
			t.Fatalf("Content-Type = %q, want %q", got, tc.expectedContentType)
		}
	}
	if tc.expectedBody != "" && string(body) != tc.expectedBody {
		t.Fatalf("body = %q, want %q", body, tc.expectedBody)
	}
	if tc.expectedJSONBody != "" {
		assertJSONEqual(t, string(body), tc.expectedJSONBody)
	}
}

func assertJSONEqual(t *testing.T, actual, expected string) {
	t.Helper()
	var actualValue any
	if err := json.Unmarshal([]byte(actual), &actualValue); err != nil {
		t.Fatalf("actual JSON %q is invalid: %v", actual, err)
	}
	var expectedValue any
	if err := json.Unmarshal([]byte(expected), &expectedValue); err != nil {
		t.Fatalf("expected JSON %q is invalid: %v", expected, err)
	}
	if !reflect.DeepEqual(actualValue, expectedValue) {
		t.Fatalf("JSON body = %s, want %s", actual, expected)
	}
}
