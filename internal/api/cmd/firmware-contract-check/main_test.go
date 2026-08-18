package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalapi "github.com/lxdb/busylib-go/internal/api"
)

func TestCheckFramesVerifiesCanonicalFactsAndDetectsBGRDrift(t *testing.T) {
	contract, err := internalapi.LoadContractFile("../../testdata/firmware-contract.json")
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	root := t.TempDir()
	files := map[string]string{
		"applications/services/web_server/http_api/api_streaming.c": `
http_api_streaming_single_frame_callback
FRONT_DISPLAY_BUF_SIZE
BACK_DISPLAY_BUF_SIZE / 2
color_buf_l8_to_l4
`,
		"applications/services/state_publisher/screen_streamer.c": `
frame_data_init
get_frame
ScreenStreamerPixelFormatR8G8B8
ScreenStreamerPixelFormatL4
FRONT_DISPLAY_W
FRONT_DISPLAY_H
BACK_DISPLAY_W
BACK_DISPLAY_H
const uint8_t blk_size = instance->display_id == GuiDisplayIdFront ? 3 : 2;
ScreenStreamerCompressionRLE
ScreenStreamerCompressionPlain
`,
		"applications/services/state_publisher/subscriptions.c": `
collect_frame
ScreenStreamerCompressionPlain] = BSB_Frame_Encoding_PLAIN
ScreenStreamerCompressionRLE] = BSB_Frame_Encoding_RUN_LENGTH
ScreenStreamerPixelFormatR8G8B8] = BSB_Frame_PixelFormat_RGB888
ScreenStreamerPixelFormatL8] = BSB_Frame_PixelFormat_L8
ScreenStreamerPixelFormatL4] = BSB_Frame_PixelFormat_L4
GuiDisplayIdFront] = BSB_Frame_Screen_FRONT
GuiDisplayIdBack] = BSB_Frame_Screen_BACK
`,
		"lib/toolbox/color.c": `
color_buf_l8_to_l4
(src_u8[src_i] >> 4) | (src_u8[src_i + 1] & 0xF0)
`,
		"applications/services/front_display/front_display.h": `
#define FRONT_DISPLAY_W (72)
#define FRONT_DISPLAY_H (16)
#define FRONT_DISPLAY_BPP (24)
`,
		"applications/services/back_display/back_display.h": `
#define BACK_DISPLAY_W (160)
#define BACK_DISPLAY_H (80)
#define BACK_DISPLAY_BPP (8)
`,
		"applications/services/gui/modules/canvas.c": `
lv_canvas_set_px_no_invalidate
data[2] = color.red;
data[1] = color.green;
data[0] = color.blue;
`,
	}
	writeFirmwareFixture(t, root, files)

	if err := checkFrames(root, contract.Frames, make(map[string][]byte)); err != nil {
		t.Fatalf("checkFrames: %v", err)
	}

	canvasPath := filepath.Join(root, "applications/services/gui/modules/canvas.c")
	canvas := strings.ReplaceAll(files["applications/services/gui/modules/canvas.c"], "data[0] = color.blue;", "data[0] = color.red;")
	if err := os.WriteFile(canvasPath, []byte(canvas), 0o600); err != nil {
		t.Fatalf("rewrite canvas fixture: %v", err)
	}
	if err := checkFrames(root, contract.Frames, make(map[string][]byte)); err == nil {
		t.Fatal("checkFrames accepted changed RGB888 byte order")
	}
}

func TestCheckLogDumpVerifiesAPI25FilenameAndJSONResponse(t *testing.T) {
	root := t.TempDir()
	const source = `
#define HTTP_API_LOG_DUMP_FILENAME_MAX 64
int filename_length = mg_http_get_var(&msg->query, "filename", filename, sizeof(filename));
http_api_log_filename_is_valid(filename, filename_length)
path_concat(STORAGE_EXT_PATH_PREFIX, filename, full_path_builder);
furi_string_cat_printf(full_path_builder, ".txt");
MG_REPLY_OK_BODY(conn, "{\"result\":\"OK\",\"path\":\"%s\"}\n", result_path);
MG_REPLY_ERROR(conn, 508, "Failed to dump logs.");
`
	path := "applications/services/web_server/http_api/api_log.c"
	writeFirmwareFixture(t, root, map[string]string{path: source})

	if err := checkLogDump(root, make(map[string][]byte)); err != nil {
		t.Fatalf("checkLogDump: %v", err)
	}

	drifted := strings.Replace(source, `"filename"`, `"path"`, 1)
	if err := os.WriteFile(filepath.Join(root, path), []byte(drifted), 0o600); err != nil {
		t.Fatalf("rewrite log dump fixture: %v", err)
	}
	if err := checkLogDump(root, make(map[string][]byte)); err == nil {
		t.Fatal("checkLogDump accepted the API 24 path query")
	}
}

func TestCheckHTTPScreenTransportVerifiesBase64Response(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"applications/services/web_server/http_api/api_streaming.c": `MG_REPLY_IMAGE(conn, frame, frame_size);`,
		"applications/services/web_server/web_server_i.h":           `#define MG_REPLY_IMAGE(conn, image, size) mg_http_reply(conn, 200, DEFAULT_IMAGE_HEADERS, "%M", mg_print_base64, size, image)`,
	}
	writeFirmwareFixture(t, root, files)

	if err := checkHTTPScreenTransport(root, make(map[string][]byte)); err != nil {
		t.Fatalf("checkHTTPScreenTransport: %v", err)
	}

	headerPath := filepath.Join(root, "applications/services/web_server/web_server_i.h")
	drifted := strings.ReplaceAll(files["applications/services/web_server/web_server_i.h"], "mg_print_base64", "mg_print_hex")
	if err := os.WriteFile(headerPath, []byte(drifted), 0o600); err != nil {
		t.Fatalf("rewrite web server fixture: %v", err)
	}
	if err := checkHTTPScreenTransport(root, make(map[string][]byte)); err == nil {
		t.Fatal("checkHTTPScreenTransport accepted non-Base64 frame output")
	}
}

func TestCheckSnapshotsVerifiesCanonicalKeysAndAllTypedUpdateTags(t *testing.T) {
	contract, err := internalapi.LoadContractFile("../../testdata/firmware-contract.json")
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	root := t.TempDir()
	files := map[string]string{
		"applications/services/web_server/http_api/api_name.c": `http_api_name_callback "name"`,
		"applications/services/web_server/http_api/api_root.c": `http_api_root_callback \"api_semver\"`,
		"applications/services/web_server/http_api/api_status.c": `
http_api_status_callback
"device" "firmware" "system" "power"
"api_semver" "uptime" "boot_time" "auto_update_enabled"
"state" "battery_charge" "battery_voltage" "battery_current" "usb_voltage"
`,
		"applications/services/web_server/http_api/api_time.c":    `http_api_time_callback "timestamp"`,
		"applications/services/web_server/http_api/api_wifi.c":    `http_api_wifi_callback "state" "ssid" "bssid" "channel" "rssi" "security" "ip_config"`,
		"applications/services/web_server/http_api/api_display.c": `http_api_display_callback "value"`,
		"applications/services/web_server/http_api/api_audio.c":   `http_api_audio_callback "volume"`,
		"applications/services/web_server/http_api/api_ble.c":     `http_api_ble_callback "status" "address"`,
		"applications/services/web_server/http_api/api_storage.c": `http_api_storage_callback "used_bytes" "free_bytes" "total_bytes"`,
		"applications/services/state_publisher/subscriptions.c": `
state_publisher_collect_all
BSB_State_StateUpdate_device_name_tag
BSB_State_StateUpdate_power_tag
BSB_State_StateUpdate_brightness_tag
BSB_State_StateUpdate_audio_volume_tag
BSB_State_StateUpdate_wifi_tag
BSB_State_StateUpdate_update_state_tag
BSB_State_StateUpdate_update_check_tag
BSB_State_StateUpdate_timezone_tag
BSB_State_StateUpdate_matter_tag
BSB_State_StateUpdate_frame_tag
BSB_State_StateUpdate_input_tag
BSB_State_StateUpdate_timer_tag
BSB_State_StateUpdate_ble_tag
BSB_State_StateUpdate_auto_update_state_tag
BSB_State_StateUpdate_timer_profiles_tag
`,
	}
	writeFirmwareFixture(t, root, files)

	if err := checkSnapshots(root, contract, make(map[string][]byte)); err != nil {
		t.Fatalf("checkSnapshots: %v", err)
	}

	namePath := filepath.Join(root, "applications/services/web_server/http_api/api_name.c")
	if err := os.WriteFile(namePath, []byte(`http_api_name_callback "device"`), 0o600); err != nil {
		t.Fatalf("rewrite name fixture: %v", err)
	}
	if err := checkSnapshots(root, contract, make(map[string][]byte)); err == nil {
		t.Fatal("checkSnapshots accepted a changed canonical name key")
	}
}

func TestCheckOptionalToolsVerifiesCLIAndMediaFacts(t *testing.T) {
	contract, err := internalapi.LoadContractFile("../../testdata/firmware-contract.json")
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	root := t.TempDir()
	files := map[string]string{
		"applications/services/cli_socket/cli_socket.c":                                  `#define CLI_SOCKET_PORT 23`,
		"applications/services/usb_network/settings/usb_network_settings_interface_v1.c": `.bytes = {10, 0, 4, 20}`,
		"lib/cli/shell/cli_shell_line.c":                                                 `snprintf(buf, length - 1, "%s>: ", prompt ? prompt : "");`,
		"applications/services/power/power_cli.c":                                        `power_cli_command power_cli_reboot "sw"`,
		"targets/f21/config/lv_conf.h":                                                   `#define LV_USE_LODEPNG 1`,
		"applications/services/web_server/http_api/api_display.c":                        `api_display_validate_image header.w > display_parameters->width header.h > display_parameters->height`,
		"applications/services/audio/audio.c":                                            `#define AUDIO_SAMPLE_RATE (44100) int16_t buffer storage_file_read`,
		"applications/services/audio/audio.h":                                            `audio_play_file Header: none Channels: 1 Rate: 44100 Hz Bits: 16bit LE`,
	}
	for _, command := range contract.OptionalTools.CLI.Commands {
		files[command.SourceFile] += ` name="` + command.Name + `" ` + command.SourceSymbol
	}
	writeFirmwareFixture(t, root, files)

	if err := checkOptionalTools(root, contract.OptionalTools, make(map[string][]byte)); err != nil {
		t.Fatalf("checkOptionalTools: %v", err)
	}

	promptPath := filepath.Join(root, "lib/cli/shell/cli_shell_line.c")
	if err := os.WriteFile(promptPath, []byte(`snprintf(buf, length - 1, "%s> ", prompt ? prompt : "");`), 0o600); err != nil {
		t.Fatalf("rewrite prompt fixture: %v", err)
	}
	if err := checkOptionalTools(root, contract.OptionalTools, make(map[string][]byte)); err == nil {
		t.Fatal("checkOptionalTools accepted a changed CLI prompt")
	}
}

func TestCheckRemoteMQTTVerifiesCanonicalFactsAndDetectsBlocklistDrift(t *testing.T) {
	contract, err := internalapi.LoadContractFile("../../testdata/firmware-contract.json")
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	root := t.TempDir()
	files := map[string]string{
		"applications/services/mqtt/mqtt_connection.c": `#define MQTT_VERSION (5)`,
		"applications/services/mqtt/mqtt_i.h": `
#define MQTT_API_VERSION "v1"
#define MQTT_ROOT_TOPIC_SESSION "sessions"
#define MQTT_DIRECTION_UP "up"
#define MQTT_DIRECTION_DOWN "down"
`,
		"applications/services/mqtt/mqtt_subscription.c": `
mqtt_make_topic_path
root = MQTT_ROOT_TOPIC_SESSION;
furi_string_printf(out, "%s/%s/%s/%s/%s", root, id, dir, MQTT_API_VERSION, topic);
MQTT_DIRECTION_DOWN
MQTT_DIRECTION_UP
`,
		"applications/services/mqtt/mqtt_message.c": `
mqtt_message_trim_response_topic
MQTT_ROOT_TOPIC_SESSION "/*/" MQTT_DIRECTION_UP "/" MQTT_API_VERSION "/#"
`,
		"applications/services/mqtt/modules/mqtt_http_proxy.c": `
#define HTTP_HOST "http://127.0.0.1"
#define HTTP_URI_API_PREFIX "/api/"
#define HTTP_CONN_TIMEOUT_MS (5000)
#define SUB_QOS (MqttQosExactlyOnce)
#define SUB_TOPIC "http-request"
[MqttHttpProxyMethodIdGet] = "GET"
[MqttHttpProxyMethodIdPost] = "POST"
[MqttHttpProxyMethodIdPut] = "PUT"
[MqttHttpProxyMethodIdDelete] = "DELETE"
mqtt_http_proxy_blocklist
{ .name = "update", .id = MqttHttpProxyMethodIdPost }
{ .name = "account", .id = MqttHttpProxyMethodIdDelete }
{ .name = "account/link", .id = MqttHttpProxyMethodIdPost }
{ .name = "account/backend", .id = MqttHttpProxyMethodIdPut }
{ .name = "wifi/connect", .id = MqttHttpProxyMethodIdPost }
{ .name = "wifi/disconnect", .id = MqttHttpProxyMethodIdPost }
{ .name = "wifi/networks", .id = MqttHttpProxyMethodIdGet }
MqttPropertyTypeResponseTopic
MqttPropertyTypeCorrelationData
mqtt_http_proxy_request_requires_response
mqtt_publish_ex
MqttQosAtLeastOnce
HTTP/1.1 422 Unprocessable Entity
mqtt_http_proxy_is_websocket_upgrade
`,
		"applications/services/mqtt/modules/mqtt_streaming.c": `
#define SUB_QOS (MqttQosAtLeastOnce)
#define PUB_QOS (MqttQosAtMostOnce)
#define SUB_TOPIC "stream-request"
#define API_QUEUE_SIZE (4)
#define FRAME_PERIOD_MS (500)
#define EXPIRY_INTERVAL_DEFAULT_S (60)
mqtt_streaming_message_callback
MqttPropertyTypeExpiryInterval
MqttPropertyTypeResponseTopic
.type = data_size ? MqttStreamingApiMessageTypeStart : MqttStreamingApiMessageTypeStop
"message_limits"
"max_count"
"interval_s"
state_publisher_handle
response_topic
state_publisher_add_transport
`,
		"applications/services/state_publisher/state_publisher.c": `
state_publisher_add_transport(void) {
    return;
}
`,
	}
	writeFirmwareFixture(t, root, files)

	if err := checkRemoteMQTT(root, contract.Remote, make(map[string][]byte)); err != nil {
		t.Fatalf("checkRemoteMQTT: %v", err)
	}

	httpPath := filepath.Join(root, "applications/services/mqtt/modules/mqtt_http_proxy.c")
	drifted := strings.Replace(files["applications/services/mqtt/modules/mqtt_http_proxy.c"], `.name = "update", .id = MqttHttpProxyMethodIdPost`, `.name = "update", .id = MqttHttpProxyMethodIdPut`, 1)
	if err := os.WriteFile(httpPath, []byte(drifted), 0o600); err != nil {
		t.Fatalf("rewrite remote HTTP fixture: %v", err)
	}
	if err := checkRemoteMQTT(root, contract.Remote, make(map[string][]byte)); err == nil {
		t.Fatal("checkRemoteMQTT accepted a changed firmware blocklist")
	}

	writeFirmwareFixture(t, root, map[string]string{
		"applications/services/mqtt/modules/mqtt_http_proxy.c": files["applications/services/mqtt/modules/mqtt_http_proxy.c"],
		"applications/services/state_publisher/state_publisher.c": `
state_publisher_add_transport(void) {
    state_publisher_send_complete_snapshot();
}
`,
	})
	if err := checkRemoteMQTT(root, contract.Remote, make(map[string][]byte)); err == nil {
		t.Fatal("checkRemoteMQTT accepted an implicit snapshot added to state_publisher_add_transport")
	}
}

func writeFirmwareFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
}
