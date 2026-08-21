package busylib

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	framepkg "github.com/lxdb/busylib-go/frame"
	internalapi "github.com/lxdb/busylib-go/internal/api"
)

func TestHTTPServiceOperationCoverageMatchesFirmwareContract(t *testing.T) {
	contract, err := internalapi.LoadContractFile("internal/api/testdata/firmware-contract.json")
	if err != nil {
		t.Fatalf("read firmware contract: %v", err)
	}

	covered := map[string]string{
		"GET /api/status/ws": "phase 6 local status stream",
	}
	for _, tc := range serviceRequestCases(t) {
		operationID := tc.operationID()
		if operationID == "" {
			t.Fatalf("%s does not declare a firmware operation ID", tc.name)
		}
		if existing, ok := covered[operationID]; ok {
			t.Fatalf("operation %s is covered by both %s and %s", operationID, existing, tc.name)
		}
		covered[operationID] = tc.name
	}

	contractIDs := make(map[string]struct{}, len(contract.Operations))
	for _, operation := range contract.Operations {
		id := operation.ID()
		contractIDs[id] = struct{}{}
		if _, ok := covered[id]; !ok {
			t.Fatalf("operation %s is not covered by a service method or explicit phase owner", id)
		}
	}
	for operationID := range covered {
		if _, ok := contractIDs[operationID]; !ok {
			t.Fatalf("operation %s is covered but is not in the firmware contract", operationID)
		}
	}
	if len(covered) != len(contract.Operations) {
		t.Fatalf("covered operation count = %d, want %d", len(covered), len(contract.Operations))
	}
	if got := covered["GET /api/status/ws"]; got != "phase 6 local status stream" {
		t.Fatalf("GET /api/status/ws coverage = %q, want phase 6 owner", got)
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
				if got.APISemVer != "25.0.0" {
					t.Fatalf("APISemVer = %q", got.APISemVer)
				}
				return nil
			},
			method:   http.MethodGet,
			path:     "/api/version",
			response: `{"api_semver":"25.0.0"}`,
		},
		jsonGetCase("system status", "/api/status", func(ctx context.Context, client *Client) error {
			got, err := client.System().Status(ctx)
			if err != nil {
				return err
			}
			if got.System.APISemVer != "25.0.0" {
				t.Fatalf("system api_semver = %q", got.System.APISemVer)
			}
			return nil
		}, `{"system":{"api_semver":"25.0.0","uptime":"00d","boot_time":1,"auto_update_enabled":true}}`),
		jsonGetCase("system device status", "/api/status/device", func(ctx context.Context, client *Client) error {
			_, err := client.System().DeviceStatus(ctx)
			return err
		}, `{"serial_number":"203","usb_mac":"00:11","otp_valid":true,"firmware_security":"secure"}`),
		jsonGetCase("system firmware status", "/api/status/firmware", func(ctx context.Context, client *Client) error {
			_, err := client.System().FirmwareStatus(ctx)
			return err
		}, `{"version":"1.0.0","target":22,"branch":"main","build_date":"2026-01-01","commit_hash":"abc","intercom_version":"2.0.0"}`),
		jsonGetCase("system system status", "/api/status/system", func(ctx context.Context, client *Client) error {
			_, err := client.System().SystemStatus(ctx)
			return err
		}, `{"api_semver":"25.0.0","uptime":"00d","boot_time":1,"auto_update_enabled":true}`),
		jsonGetCase("system power status", "/api/status/power", func(ctx context.Context, client *Client) error {
			_, err := client.System().PowerStatus(ctx)
			return err
		}, `{"state":"charged","battery_charge":99,"battery_voltage":4183,"battery_current":-1,"usb_voltage":4843}`),
		jsonGetCase("system transport", "/api/transport", func(ctx context.Context, client *Client) error {
			_, err := client.System().Transport(ctx)
			return err
		}, `{"type":"usb"}`),
		{
			name: "system dump log",
			call: func(ctx context.Context, client *Client) error {
				got, err := client.System().DumpLog(ctx, "physical_test")
				if err != nil {
					return err
				}
				if got.Result != "OK" || got.Path != "/ext/physical_test.txt" {
					t.Fatalf("DumpLog response = %#v", got)
				}
				return nil
			},
			method:   http.MethodPost,
			path:     "/api/log_dump",
			query:    "filename=physical_test",
			response: `{"result":"OK","path":"/ext/physical_test.txt"}`,
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
		successJSONCase("display draw", http.MethodPost, "/api/display/draw", "", `{"application_name":"app","priority":50,"elements":[{"id":"text","type":"text","text":"Hi","font":"normal"}]}`, func(ctx context.Context, client *Client) error {
			return client.Display().Draw(ctx, DisplayElements{
				ApplicationName: "app",
				Priority:        DefaultDisplayPriority,
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
			response: "AQID",
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
			return client.WiFi().Connect(ctx, ConnectRequestConfig{SSID: "ssid", Password: "pass", Security: WiFiSecurityWPA3, IPConfig: WiFiConnectIPConfig{IPMethod: WiFiIPMethodDHCP}})
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
			return client.SmartHome().SetSwitchState(ctx, SmartHomeSwitchUpdate{State: boolPtr(false), Startup: SmartHomeSwitchStartupLast})
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
		jsonGetCase("update autoupdate", "/api/update/autoupdate", func(ctx context.Context, client *Client) error { _, err := client.Update().AutoUpdate(ctx); return err }, `{"is_enabled":true,"interval_start":"00:00","interval_end":"08:00"}`),
		successJSONCase("update set autoupdate", http.MethodPost, "/api/update/autoupdate", "", `{"is_enabled":true,"interval_start":"00:00","interval_end":"08:00"}`, func(ctx context.Context, client *Client) error {
			enabled := true
			return client.Update().SetAutoUpdate(ctx, AutoUpdateSettings{IsEnabled: &enabled, IntervalStart: "00:00", IntervalEnd: "08:00"})
		}, okJSON),
	}
}

func TestHTTPServicesSendExpectedRequests(t *testing.T) {
	ctx := context.Background()
	for _, tc := range serviceRequestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			requestErr := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestErr <- serviceRequestError(tc, r)
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
				WithVersionNegotiation(VersionNegotiationDisabled),
				WithRequestIDGenerator(fixedRequestID("rid-service")),
			)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			if err := tc.call(ctx, client); err != nil {
				t.Fatalf("service call: %v", err)
			}
			if err := <-requestErr; err != nil {
				t.Fatalf("request contract: %v", err)
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
		SSID:     "ssid",
		Security: WiFiSecurityWPA3,
		IPConfig: WiFiConnectIPConfig{
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

	assertJSONEqual(t, string(data), `{"ssid":"ssid","password":"","security":"WPA3","ip_config":{"ip_method":"static","address":"192.0.2.10","mask":"255.255.255.0","gateway":"192.0.2.1"}}`)
}

func TestDisplayHelperConstructorsMarshalAndValidate(t *testing.T) {
	request := NewDisplayElements("app",
		NewTextElement("text", "Hi", FontNormal),
		NewAssetImageElement("image", "logo.png"),
		NewStockImageElement("stock_image", "shared/logo.png"),
		NewAssetAnimationElement("animation", "busy.anim"),
		NewStockAnimationElement("stock_animation", "shared/spin.anim"),
		NewCountdownElement("countdown", "1700000000", CountdownTimeLeft, CountdownShowHoursWhenNonZero),
		NewRectangleElement("rectangle", 12, 4),
	)

	if request.Priority != DefaultDisplayPriority {
		t.Fatalf("Priority = %d, want default priority", request.Priority)
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	assertJSONEqual(t, string(body), `{
		"application_name":"app",
		"priority":50,
		"elements":[
			{"id":"text","display":"front","type":"text","text":"Hi","font":"normal"},
			{"id":"image","display":"front","type":"image","path":"logo.png"},
			{"id":"stock_image","display":"front","type":"image","stock_path":"shared/logo.png"},
			{"id":"animation","display":"front","type":"animation","path":"busy.anim"},
			{"id":"stock_animation","display":"front","type":"animation","stock_path":"shared/spin.anim"},
			{"id":"countdown","display":"front","type":"countdown","timestamp":"1700000000","direction":"time_left","show_hours":"when_non_zero"},
			{"id":"rectangle","display":"front","type":"rectangle","width":12,"height":4}
		]
	}`)
}

func TestFirmwareDisplayTextAcceptsUTF8AndSanitizationIsOptIn(t *testing.T) {
	raw := " Hello\t\U0001F31F\nBUSY  Bar "
	if got := SanitizeDisplayText(raw); got != "Hello BUSY Bar" {
		t.Fatalf("SanitizeDisplayText = %q", got)
	}

	err := (DisplayElements{
		ApplicationName: "app",
		Priority:        DefaultDisplayPriority,
		Elements: []DisplayElement{
			NewTextElement("text", raw, FontNormal),
		},
	}).Validate()
	if err != nil {
		t.Fatalf("raw firmware-valid display text Validate: %v", err)
	}

	err = (DisplayElements{
		ApplicationName: "app",
		Priority:        DefaultDisplayPriority,
		Elements: []DisplayElement{
			NewTextElement("text", SanitizeDisplayText(raw), FontNormal),
		},
	}).Validate()
	if err != nil {
		t.Fatalf("sanitized display text Validate: %v", err)
	}
}

func TestProductValidatorsAcceptDocumentedBoundaries(t *testing.T) {
	color, err := NormalizeColor("#aabbccdd")
	if err != nil {
		t.Fatalf("NormalizeColor: %v", err)
	}
	if color != "#AABBCCDD" {
		t.Fatalf("NormalizeColor = %q, want uppercase #RRGGBBAA", color)
	}

	zero := 0
	request := DisplayElements{
		ApplicationName:      "app-1",
		Priority:             100,
		LEDNotificationColor: "#00AAFFFF",
		Elements: []DisplayElement{
			TextElement{
				BaseDisplayElement: BaseDisplayElement{
					ID:      "text_1",
					Timeout: &zero,
					X:       intPtr(math.MinInt16),
					Y:       intPtr(math.MaxInt16),
					Display: DisplayFront,
					Align:   DisplayAlignCenter,
				},
				Text:  "Hello!",
				Font:  FontGlobal,
				Color: "#FFFFFFFF",
				Width: 1,
			},
			ImageElement{
				BaseDisplayElement: BaseDisplayElement{ID: "image.1"},
				StockPath:          "shared/chime.snd",
				Opacity:            intPtr(0),
			},
			RectangleElement{
				BaseDisplayElement: BaseDisplayElement{ID: "rect"},
				Width:              1,
				Height:             1,
				Radius:             0,
				Fill:               RectangleFillGradientH,
				FillColors:         []string{"#FFFFFFFF", "#00000000"},
				BorderWidth:        intPtr(0),
				BorderColor:        "#FFFFFFFF",
			},
		},
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("DisplayElements.Validate: %v", err)
	}

	if err := (PlayAudio{ApplicationName: "app", Path: "tone.snd"}).Validate(); err != nil {
		t.Fatalf("PlayAudio.Validate: %v", err)
	}
	if err := (AutoUpdateSettings{IntervalStart: "00:00", IntervalEnd: "23:59"}).Validate(); err != nil {
		t.Fatalf("AutoUpdateSettings.Validate: %v", err)
	}
	if err := (AutoUpdateSettings{IntervalStart: "8:00", IntervalEnd: "9:30"}).Validate(); err != nil {
		t.Fatalf("AutoUpdateSettings non-padded firmware clock Validate: %v", err)
	}
	if err := (ConnectRequestConfig{
		SSID:     "ssid",
		Security: WiFiSecurityWPA3,
		IPConfig: WiFiConnectIPConfig{
			IPMethod: WiFiIPMethodStatic,
			Address:  "192.0.2.10",
			Mask:     "255.255.255.0",
			Gateway:  "192.0.2.1",
		},
	}).Validate(); err != nil {
		t.Fatalf("ConnectRequestConfig.Validate: %v", err)
	}
}

func TestFirmware24_4ValidationShapes(t *testing.T) {
	if err := validateTimestamp("20260201T114230Z"); err != nil {
		t.Fatalf("compact firmware timestamp: %v", err)
	}
	if err := validateTimezone("~UTC"); err != nil {
		t.Fatalf("timezone should be left to firmware membership validation: %v", err)
	}

	if err := (ConnectRequestConfig{
		SSID:     " ",
		Security: WiFiSecurityOpen,
		IPConfig: WiFiConnectIPConfig{IPMethod: WiFiIPMethodDHCP},
	}).Validate(); err != nil {
		t.Fatalf("firmware-valid open Wi-Fi request: %v", err)
	}
	if err := (ConnectRequestConfig{
		SSID:     "ssid",
		Security: WiFiSecurityUnsupported,
		IPConfig: WiFiConnectIPConfig{IPMethod: WiFiIPMethodDHCP},
	}).Validate(); err == nil {
		t.Fatal("Unsupported is response-only and must be rejected for connect")
	}

	updateBody, err := json.Marshal(SmartHomeSwitchUpdate{Startup: SmartHomeSwitchStartupLast})
	if err != nil {
		t.Fatalf("marshal Matter update: %v", err)
	}
	assertJSONEqual(t, string(updateBody), `{"startup":"last"}`)
}

func TestBusyFirmwareValidation(t *testing.T) {
	paused := false
	interval := 0
	total := int64(300_000)
	left := int64(240_000)
	snapshot := BusySnapshot{
		Snapshot: BusySnapshotData{
			Type:                       BusySnapshotInterval,
			CardID:                     "00000000-0000-0000-0000-000000000000",
			IsPaused:                   &paused,
			CurrentInterval:            &interval,
			CurrentIntervalTimeTotalMS: &total,
			CurrentIntervalTimeLeftMS:  &left,
			IntervalSettings: &BusyTimerIntervalSettings{
				IntervalWorkMS:          300_000,
				IntervalRestMS:          300_000,
				IntervalWorkCyclesCount: 2,
			},
			BusyBarSettings: BusyBarSettings{Theme: "busy"},
		},
		SnapshotTimestampMS: 1,
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("valid firmware interval snapshot: %v", err)
	}

	invalid := snapshot
	tooLong := int64(300_001)
	invalid.Snapshot.CurrentIntervalTimeLeftMS = &tooLong
	if err := invalid.Validate(); err == nil {
		t.Fatal("interval time left greater than total must be rejected")
	}

	profile := BusyProfile{
		ID:            "00000000-0000-0000-0000-000000000000",
		Title:         "Focus",
		TimerSettings: BusyTimerSettings{Type: BusyTimerInfinite},
		BusyBarSettings: BusyBarSettings{
			Theme: "busy",
		},
	}
	if err := profile.Validate(); err != nil {
		t.Fatalf("valid firmware profile: %v", err)
	}
}

func TestDisplayWarningsDoNotBlockContractValidPlacement(t *testing.T) {
	request := DisplayElements{
		ApplicationName: "app",
		Priority:        DefaultDisplayPriority,
		Elements: []DisplayElement{
			TextElement{
				BaseDisplayElement: BaseDisplayElement{
					ID:      "text",
					X:       intPtr(200),
					Y:       intPtr(60),
					Display: DisplayFront,
				},
				Text: "Hi",
				Font: FontNormal,
			},
		},
	}

	if err := request.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	warnings := request.Warnings()
	if len(warnings) == 0 {
		t.Fatal("expected placement warning")
	}
	if warnings[0].Field == "" || warnings[0].Message == "" {
		t.Fatalf("warning should include field and message: %#v", warnings[0])
	}
}

func TestDisplayValidationAcceptsPointerElements(t *testing.T) {
	request := DisplayElements{
		ApplicationName: "app",
		Priority:        DefaultDisplayPriority,
		Elements: []DisplayElement{
			&TextElement{BaseDisplayElement: BaseDisplayElement{ID: "text"}, Text: "Hi", Font: FontNormal},
			&ImageElement{BaseDisplayElement: BaseDisplayElement{ID: "image"}, Path: "image.png"},
			&AnimationElement{BaseDisplayElement: BaseDisplayElement{ID: "animation"}, Path: "animation.gif"},
			&CountdownElement{BaseDisplayElement: BaseDisplayElement{ID: "countdown"}, Timestamp: "1769437579", Direction: CountdownTimeLeft, ShowHours: CountdownShowHoursAlways},
			&RectangleElement{BaseDisplayElement: BaseDisplayElement{ID: "rectangle"}, Width: 1, Height: 1},
		},
	}

	if err := request.Validate(); err != nil {
		t.Fatalf("Validate pointer elements: %v", err)
	}
}

func TestDisplayWarningsInspectPointerElements(t *testing.T) {
	request := DisplayElements{
		ApplicationName: "app",
		Priority:        DefaultDisplayPriority,
		Elements: []DisplayElement{
			&TextElement{
				BaseDisplayElement: BaseDisplayElement{
					ID:      "text",
					X:       intPtr(200),
					Y:       intPtr(60),
					Display: DisplayFront,
				},
				Text: "Hi",
				Font: FontNormal,
			},
		},
	}

	if err := request.Validate(); err != nil {
		t.Fatalf("Validate pointer element: %v", err)
	}
	if warnings := request.Warnings(); len(warnings) == 0 {
		t.Fatal("expected pointer element placement warning")
	}
}

func TestDisplayValidationRejectsTypedNilPointerElements(t *testing.T) {
	var text *TextElement
	request := DisplayElements{
		ApplicationName: "app",
		Priority:        DefaultDisplayPriority,
		Elements:        []DisplayElement{text},
	}

	if err := request.Validate(); err == nil {
		t.Fatal("expected typed nil pointer validation error")
	}
}

func TestProductValidatorsRejectInvalidInputs(t *testing.T) {
	invalid := []struct {
		name string
		err  error
	}{
		{"color", func() error { _, err := NormalizeColor("#fff"); return err }()},
		{"draw priority above firmware maximum", (DisplayElements{Priority: 101, Elements: []DisplayElement{TextElement{BaseDisplayElement: BaseDisplayElement{ID: "text"}, Text: "Hi", Font: FontNormal}}}).Validate()},
		{"draw empty elements", (DisplayElements{ApplicationName: "app", Priority: DefaultDisplayPriority}).Validate()},
		{"text invalid font", (DisplayElements{ApplicationName: "app", Priority: DefaultDisplayPriority, Elements: []DisplayElement{TextElement{BaseDisplayElement: BaseDisplayElement{ID: "text"}, Text: "Hi", Font: Font("huge")}}}).Validate()},
		{"image duplicate source", (DisplayElements{ApplicationName: "app", Priority: DefaultDisplayPriority, Elements: []DisplayElement{ImageElement{BaseDisplayElement: BaseDisplayElement{ID: "image"}, Path: "a.png", StockPath: "shared/a.png"}}}).Validate()},
		{"audio missing source", (PlayAudio{ApplicationName: "app"}).Validate()},
		{"autoupdate time", (AutoUpdateSettings{IntervalStart: "24:00"}).Validate()},
		{"wifi ssid", (ConnectRequestConfig{}).Validate()},
		{"wifi static ip", (ConnectRequestConfig{SSID: "ssid", Security: WiFiSecurityWPA3, IPConfig: WiFiConnectIPConfig{IPMethod: WiFiIPMethodStatic, Address: "192.0.2.10"}}).Validate()},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestAudioHelperMethodsSendDocumentedPayloads(t *testing.T) {
	ctx := context.Background()
	calls := []struct {
		method string
		path   string
		query  string
		body   string
	}{
		{method: http.MethodPost, path: "/api/audio/play", body: `{"application_name":"app","path":"tone.snd"}`},
		{method: http.MethodPost, path: "/api/audio/play", body: `{"application_name":"app","stock_path":"shared/tone.snd"}`},
		{method: http.MethodPost, path: "/api/audio/volume", query: "volume=30"},
		{method: http.MethodPost, path: "/api/audio/volume", query: "silent=1&volume=25"},
	}
	var index int
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if index >= len(calls) {
				t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
			}
			want := calls[index]
			index++
			if r.Method != want.method || r.URL.Path != want.path || r.URL.RawQuery != want.query {
				t.Fatalf("request = %s %s?%s, want %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery, want.method, want.path, want.query)
			}
			if want.body != "" {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				assertJSONEqual(t, string(body), want.body)
			}
			return jsonResponse(http.StatusOK, map[string]string{"result": "OK"}), nil
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.Audio().PlayAsset(ctx, "app", "tone.snd"); err != nil {
		t.Fatalf("PlayAsset: %v", err)
	}
	if err := client.Audio().PlayStock(ctx, "app", "shared/tone.snd"); err != nil {
		t.Fatalf("PlayStock: %v", err)
	}
	if err := client.Audio().SetVolume(ctx, SetAudioVolumeRequest{Volume: 30}); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if err := client.Audio().SetVolumeSilently(ctx, 25); err != nil {
		t.Fatalf("SetVolumeSilently: %v", err)
	}
	if index != len(calls) {
		t.Fatalf("requests = %d, want %d", index, len(calls))
	}
}

func TestDisplayGlobalClearAndFrontScreenFetch(t *testing.T) {
	var calls int
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			switch calls {
			case 1:
				if r.Method != http.MethodDelete || r.URL.Path != "/api/display/draw" || r.URL.RawQuery != "" {
					t.Fatalf("global clear request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
				}
				return jsonResponse(http.StatusOK, map[string]string{"result": "OK"}), nil
			case 2:
				if r.Method != http.MethodGet || r.URL.Path != "/api/screen" || r.URL.RawQuery != "display=0" {
					t.Fatalf("front screen request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     http.Header{"Content-Type": []string{"image/bmp"}},
					Body:       io.NopCloser(strings.NewReader("AQID")),
				}, nil
			default:
				t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
				return nil, nil
			}
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.Display().Clear(context.Background(), ""); err != nil {
		t.Fatalf("Clear global: %v", err)
	}
	frame, err := client.Display().Screen(context.Background(), 0)
	if err != nil {
		t.Fatalf("Screen front: %v", err)
	}
	if !bytes.Equal(frame, []byte{1, 2, 3}) {
		t.Fatalf("front frame = %v", frame)
	}
}

func TestDisplayScreenFrameDecodesHTTPResponse(t *testing.T) {
	raw := make([]byte, framepkg.FrontWidth*framepkg.FrontHeight*3)
	raw[0], raw[1], raw[2] = 0x11, 0x22, 0x33
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodGet || r.URL.Path != "/api/screen" || r.URL.RawQuery != "display=0" {
				t.Fatalf("screen request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
				Body:       io.NopCloser(strings.NewReader(base64.StdEncoding.EncodeToString(raw))),
			}, nil
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	payload, err := client.Display().Screen(context.Background(), 0)
	if err != nil {
		t.Fatalf("Screen: %v", err)
	}
	value, err := framepkg.FromHTTP(0, payload)
	if err != nil {
		t.Fatalf("FromHTTP: %v", err)
	}
	rgba, err := value.RGBA()
	if err != nil {
		t.Fatalf("RGBA: %v", err)
	}
	if pixel := rgba.RGBAAt(0, 0); pixel.R != 0x33 || pixel.G != 0x22 || pixel.B != 0x11 || pixel.A != 0xff {
		t.Fatalf("first pixel = %#v", pixel)
	}
}

func TestDisplayScreenRejectsInvalidFirmwareBase64Payload(t *testing.T) {
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     http.Header{"Content-Type": []string{"image/bmp"}},
				Body:       io.NopCloser(strings.NewReader("not base64")),
			}, nil
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.Display().Screen(context.Background(), 0)
	var protocolErr *ProtocolError
	if !errors.As(err, &protocolErr) {
		t.Fatalf("Screen error = %T %v, want ProtocolError", err, err)
	}
}

func TestPhase5FirmwareErrorsRemainTyped(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		statusCode int
		message    string
		call       func(context.Context, *Client) error
	}{
		{
			name:       "stop with no audio",
			method:     http.MethodDelete,
			path:       "/api/audio/play",
			statusCode: http.StatusGone,
			message:    "No audio is playing",
			call: func(ctx context.Context, client *Client) error {
				return client.Audio().Stop(ctx)
			},
		},
		{
			name:       "asset payload too large",
			method:     http.MethodPost,
			path:       "/api/assets/upload",
			statusCode: http.StatusRequestEntityTooLarge,
			message:    "Payload too large",
			call: func(ctx context.Context, client *Client) error {
				return client.Assets().Upload(ctx, UploadAssetRequest{
					ApplicationName: "app",
					File:            "asset.bin",
					Body:            BytesBody([]byte("payload"), "application/octet-stream"),
				})
			},
		},
		{
			name:       "storage payload too large",
			method:     http.MethodPost,
			path:       "/api/storage/write",
			statusCode: http.StatusRequestEntityTooLarge,
			message:    "Payload too large",
			call: func(ctx context.Context, client *Client) error {
				return client.Storage().Write(ctx, WriteStorageFileRequest{
					Path: "/ext/payload.bin",
					Body: BytesBody([]byte("payload"), "application/octet-stream"),
				})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewClient(
				WithBaseURL("http://busybar.local"),
				WithVersionNegotiation(VersionNegotiationDisabled),
				WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					if r.Method != tc.method || r.URL.Path != tc.path {
						t.Fatalf("request = %s %s, want %s %s", r.Method, r.URL.Path, tc.method, tc.path)
					}
					if r.Body != nil {
						_, _ = io.Copy(io.Discard, r.Body)
						_ = r.Body.Close()
					}
					return jsonResponse(tc.statusCode, map[string]string{"error": tc.message}), nil
				})}),
			)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			err = tc.call(context.Background(), client)
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %T %v, want APIError", err, err)
			}
			if apiErr.Method != tc.method || apiErr.Path != tc.path || apiErr.StatusCode != tc.statusCode || apiErr.DeviceError != tc.message {
				t.Fatalf("APIError = method %s path %s status %d message %q", apiErr.Method, apiErr.Path, apiErr.StatusCode, apiErr.DeviceError)
			}
		})
	}
}

func TestAssetAndStorageFileHelpersUseRepeatableFileBodiesAndPreserveExtensions(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	localPath := filepath.Join(tempDir, "payload.bin")
	if err := os.WriteFile(localPath, []byte("file payload"), 0o600); err != nil {
		t.Fatalf("WriteFile fixture: %v", err)
	}

	calls := []struct {
		method string
		path   string
		query  string
	}{
		{method: http.MethodPost, path: "/api/assets/upload", query: "application_name=app&file=asset.bin"},
		{method: http.MethodPost, path: "/api/storage/write", query: "path=%2Fext%2Fpayload.unknown"},
	}
	var index int
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if index >= len(calls) {
				t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
			}
			want := calls[index]
			index++
			if r.Method != want.method || r.URL.Path != want.path || r.URL.RawQuery != want.query {
				t.Fatalf("request = %s %s?%s, want %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery, want.method, want.path, want.query)
			}
			if got := r.Header.Get("Content-Type"); got != "application/octet-stream" {
				t.Fatalf("Content-Type = %q", got)
			}
			if r.ContentLength != int64(len("file payload")) {
				t.Fatalf("ContentLength = %d", r.ContentLength)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if string(body) != "file payload" {
				t.Fatalf("body = %q", body)
			}
			return jsonResponse(http.StatusOK, map[string]string{"result": "OK"}), nil
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.Assets().UploadFile(ctx, "app", "asset.bin", localPath); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if err := client.Storage().WriteFile(ctx, "/ext/payload.unknown", localPath); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if index != len(calls) {
		t.Fatalf("requests = %d, want %d", index, len(calls))
	}
}

func TestStorageReadToStreamsResponseAndPreservesAPIErrors(t *testing.T) {
	ctx := context.Background()
	var calls int
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.Method != http.MethodGet || r.URL.Path != "/api/storage/read" || r.URL.RawQuery != "path=%2Fext%2Fpayload.bin" {
				t.Fatalf("request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
			}
			if calls == 1 {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader("streamed payload")),
				}, nil
			}
			return jsonResponse(http.StatusBadRequest, map[string]string{"error": "file not found"}), nil
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	var out bytes.Buffer
	n, err := client.Storage().ReadTo(ctx, "/ext/payload.bin", &out)
	if err != nil {
		t.Fatalf("ReadTo: %v", err)
	}
	if n != int64(len("streamed payload")) || out.String() != "streamed payload" {
		t.Fatalf("ReadTo wrote n=%d body=%q", n, out.String())
	}

	_, err = client.Storage().ReadTo(ctx, "/ext/payload.bin", io.Discard)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T %v, want APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest || apiErr.DeviceError != "file not found" {
		t.Fatalf("APIError = status %d error %q", apiErr.StatusCode, apiErr.DeviceError)
	}
}

func TestStorageResponseModelsMatchFirmwareUnsignedShapes(t *testing.T) {
	const aboveMaxInt64 uint64 = 1 << 63
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/api/storage/list":
				return jsonResponse(http.StatusOK, map[string]any{
					"list": []map[string]any{
						{"type": "file", "name": "payload.bin", "size": uint64(7)},
						{"type": "dir", "name": "assets"},
					},
				}), nil
			case "/api/storage/status":
				return jsonResponse(http.StatusOK, map[string]uint64{
					"used_bytes":  aboveMaxInt64,
					"free_bytes":  2,
					"total_bytes": aboveMaxInt64 + 2,
				}), nil
			default:
				t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
				return nil, nil
			}
		})}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	list, err := client.Storage().List(context.Background(), "/ext")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	wantList := StorageList{List: []StorageListElement{
		{Type: StorageListElementFile, Name: "payload.bin", Size: 7},
		{Type: StorageListElementDir, Name: "assets"},
	}}
	if !reflect.DeepEqual(list, wantList) {
		t.Fatalf("List = %#v, want %#v", list, wantList)
	}

	status, err := client.Storage().Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.UsedBytes != aboveMaxInt64 || status.FreeBytes != 2 || status.TotalBytes != aboveMaxInt64+2 {
		t.Fatalf("Status = %#v", status)
	}
}

func TestServiceValidationRejectsInvalidInputsBeforeNetwork(t *testing.T) {
	client, err := NewClient(
		WithBaseURL("http://busybar.local"),
		WithVersionNegotiation(VersionNegotiationDisabled),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("network should not be called for invalid request")
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
		{"dump log filename", func() error { _, err := client.System().DumpLog(ctx, "bad/name"); return err }},
		{"set access key", func() error { return client.Settings().SetHTTPAccess(ctx, HTTPAccessKey, "abc") }},
		{"set name", func() error { return client.Settings().SetName(ctx, "") }},
		{"set brightness", func() error { return client.Display().SetBrightness(ctx, "101") }},
		{"draw", func() error {
			return client.Display().Draw(ctx, DisplayElements{ApplicationName: "app", Priority: DefaultDisplayPriority})
		}},
		{"screen", func() error { _, err := client.Display().Screen(ctx, 2); return err }},
		{"audio play", func() error {
			return client.Audio().Play(ctx, PlayAudio{ApplicationName: "app", Path: "a.snd", StockPath: "shared/a.snd"})
		}},
		{"set volume", func() error { return client.Audio().SetVolume(ctx, SetAudioVolumeRequest{Volume: 101}) }},
		{"asset upload", func() error {
			return client.Assets().Upload(ctx, UploadAssetRequest{ApplicationName: "../bad", File: "data.png", Body: BytesBody([]byte("png"), "application/octet-stream")})
		}},
		{"asset delete", func() error { return client.Assets().DeleteApplicationAssets(ctx, "../bad") }},
		{"storage write", func() error {
			return client.Storage().Write(ctx, WriteStorageFileRequest{Path: "/tmp/file", Body: BytesBody([]byte("payload"), "application/octet-stream")})
		}},
		{"storage read", func() error { _, err := client.Storage().Read(ctx, "/tmp/file"); return err }},
		{"storage read to path", func() error { _, err := client.Storage().ReadTo(ctx, "/tmp/file", io.Discard); return err }},
		{"storage read to writer", func() error { _, err := client.Storage().ReadTo(ctx, "/ext/file", nil); return err }},
		{"storage rename", func() error { return client.Storage().Rename(ctx, "/ext/old", "/tmp/new") }},
		{"wifi connect", func() error { return client.WiFi().Connect(ctx, ConnectRequestConfig{}) }},
		{"input key", func() error { return client.Input().SendKey(ctx, InputKey("power")) }},
		{"smart home switch", func() error {
			return client.SmartHome().SetSwitchState(ctx, SmartHomeSwitchUpdate{Startup: SmartHomeSwitchStartup("boot")})
		}},
		{"time timestamp", func() error { return client.Time().SetTimestamp(ctx, "2025-10-02T14:30:45") }},
		{"time timezone", func() error { return client.Time().SetTimezone(ctx, "") }},
		{"update changelog", func() error { _, err := client.Update().Changelog(ctx, ""); return err }},
		{"update install", func() error { return client.Update().Install(ctx, "") }},
		{"update autoupdate", func() error { return client.Update().SetAutoUpdate(ctx, AutoUpdateSettings{IntervalEnd: "24:00"}) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			err := check.call()
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("error = %T %v, want ValidationError", err, err)
			}
		})
	}
}

func TestResponseModelsPreserveUnknownEnumStrings(t *testing.T) {
	var status WiFiStatus
	if err := json.Unmarshal([]byte(`{"state":"roaming","security":"WPA4"}`), &status); err != nil {
		t.Fatalf("unmarshal wifi status: %v", err)
	}
	if status.State != WiFiConnectionState("roaming") {
		t.Fatalf("state = %q", status.State)
	}
	if status.Security != WiFiSecurityMethod("WPA4") {
		t.Fatalf("security = %q", status.Security)
	}
}

func TestFirmwareBlockedServiceMethodsAreRejectedInRemoteMode(t *testing.T) {
	client, err := NewClient(
		WithEndpointMode(EndpointRemote),
		WithBaseURL("http://busybar.remote.invalid"),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("network should not be called for firmware-blocked remote operation")
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
		{"firmware upload", func() error {
			return client.Update().UploadPackage(ctx, BytesBody([]byte("firmware"), "application/octet-stream"))
		}},
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

func intPtr(value int) *int {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
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
	contractOperationID string
}

func (tc serviceRequestCase) withBody(body, contentType string) serviceRequestCase {
	tc.expectedBody = body
	tc.expectedContentType = contentType
	return tc
}

func (tc serviceRequestCase) withOperationID(operationID string) serviceRequestCase {
	tc.contractOperationID = operationID
	return tc
}

func (tc serviceRequestCase) operationID() string {
	if tc.contractOperationID != "" {
		return tc.contractOperationID
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

func serviceRequestError(tc serviceRequestCase, r *http.Request) error {
	if r.Method != tc.method {
		return fmt.Errorf("method = %q, want %q", r.Method, tc.method)
	}
	if r.URL.Path != tc.path {
		return fmt.Errorf("path = %q, want %q", r.URL.Path, tc.path)
	}
	if r.URL.RawQuery != tc.query {
		return fmt.Errorf("query = %q, want %q", r.URL.RawQuery, tc.query)
	}
	if got := r.Header.Get("X-API-Sem-Ver"); got != "" {
		return fmt.Errorf("X-API-Sem-Ver = %q, want absent with negotiation disabled", got)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	if got := r.Header.Get("Content-Type"); got != tc.expectedContentType {
		return fmt.Errorf("Content-Type = %q, want %q", got, tc.expectedContentType)
	}
	if tc.expectedJSONBody == "" && string(body) != tc.expectedBody {
		return fmt.Errorf("body = %q, want %q", body, tc.expectedBody)
	}
	if tc.expectedJSONBody != "" {
		if err := jsonEqualError(string(body), tc.expectedJSONBody); err != nil {
			return err
		}
	}
	return nil
}

func assertJSONEqual(t *testing.T, actual, expected string) {
	t.Helper()
	if err := jsonEqualError(actual, expected); err != nil {
		t.Fatal(err)
	}
}

func jsonEqualError(actual, expected string) error {
	var actualValue any
	if err := json.Unmarshal([]byte(actual), &actualValue); err != nil {
		return fmt.Errorf("actual JSON %q is invalid: %w", actual, err)
	}
	var expectedValue any
	if err := json.Unmarshal([]byte(expected), &expectedValue); err != nil {
		return fmt.Errorf("expected JSON %q is invalid: %w", expected, err)
	}
	if !reflect.DeepEqual(actualValue, expectedValue) {
		return fmt.Errorf("JSON body = %s, want %s", actual, expected)
	}
	return nil
}
