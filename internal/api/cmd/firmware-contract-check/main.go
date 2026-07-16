package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	internalapi "github.com/lxdb/busylib-go/internal/api"
)

var (
	apiVersionPattern = regexp.MustCompile(`#define\s+API_VERSION\s+\{\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\}`)
	defineIntPattern  = regexp.MustCompile(`#define\s+([A-Z][A-Z0-9_]*)\s+\(?\s*(\d+)\s*\)?`)
	rateLimitPattern  = regexp.MustCompile(`\.max_packet_count\s*=\s*(\d+)\s*,\s*\.period_ms\s*=\s*(\d+)`)
	blocklistPattern  = regexp.MustCompile(`(?s)\{\s*\.name\s*=\s*"([^"]+)"\s*,\s*\.id\s*=\s*(MqttHttpProxyMethodId[A-Za-z]+)\s*,?\s*\}`)
)

func main() {
	firmwareDir := flag.String("firmware-dir", "", "path to the busybar-firmware checkout")
	contractPath := flag.String("contract", "internal/api/testdata/firmware-contract.json", "path to the firmware contract receipt")
	flag.Parse()

	if *firmwareDir == "" {
		fatalf("-firmware-dir is required")
	}
	contract, err := internalapi.LoadContractFile(*contractPath)
	if err != nil {
		fatalf("load contract: %v", err)
	}
	if err := checkFirmware(*firmwareDir, contract); err != nil {
		fatalf("firmware contract drift: %v", err)
	}
	fmt.Printf("firmware contract matches %s at %s (API %s, %d operations, status stream, frames, snapshots, optional tools, and remote MQTT verified)\n", contract.Repository, contract.FirmwareCommit, contract.APIVersion, len(contract.Operations))
}

func checkFirmware(root string, contract internalapi.Contract) error {
	head, err := gitOutput(root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != contract.FirmwareCommit {
		return fmt.Errorf("HEAD = %s, audited commit = %s; audit the firmware diff before refreshing the receipt", head, contract.FirmwareCommit)
	}

	versionPath := filepath.Join(root, "applications/services/web_server/http_api/http_api.h")
	versionData, err := os.ReadFile(versionPath)
	if err != nil {
		return err
	}
	match := apiVersionPattern.FindStringSubmatch(string(versionData))
	if match == nil {
		return fmt.Errorf("API_VERSION is missing from %s", versionPath)
	}
	version := strings.Join(match[1:], ".")
	if version != contract.APIVersion {
		return fmt.Errorf("API_VERSION = %s, receipt = %s", version, contract.APIVersion)
	}

	protoTree, err := gitOutput(root, "ls-tree", "HEAD", "assets/proto")
	if err != nil {
		return err
	}
	fields := strings.Fields(protoTree)
	if len(fields) < 3 || fields[1] != "commit" {
		return fmt.Errorf("assets/proto is not a firmware gitlink: %q", protoTree)
	}
	if fields[2] != contract.ProtobufCommit {
		return fmt.Errorf("protobuf gitlink = %s, receipt = %s", fields[2], contract.ProtobufCommit)
	}

	checked := make(map[string][]byte)
	for _, operation := range contract.Operations {
		data, ok := checked[operation.SourceFile]
		if !ok {
			data, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(operation.SourceFile)))
			if err != nil {
				return fmt.Errorf("%s: %w", operation.ID(), err)
			}
			checked[operation.SourceFile] = data
		}
		if !strings.Contains(string(data), operation.SourceSymbol) {
			return fmt.Errorf("%s source symbol %q is missing from %s", operation.ID(), operation.SourceSymbol, operation.SourceFile)
		}
	}
	if err := checkStatusStream(root, contract.StatusStream, checked); err != nil {
		return err
	}
	if err := checkFrames(root, contract.Frames, checked); err != nil {
		return err
	}
	if err := checkSnapshots(root, contract, checked); err != nil {
		return err
	}
	if err := checkOptionalTools(root, contract.OptionalTools, checked); err != nil {
		return err
	}
	return checkRemoteMQTT(root, contract.Remote, checked)
}

func checkStatusStream(root string, contract internalapi.StatusStreamContract, checked map[string][]byte) error {
	for _, reference := range contract.SourceReferences {
		data, err := readFirmwareFile(root, reference.SourceFile, checked)
		if err != nil {
			return fmt.Errorf("status stream: %w", err)
		}
		if !strings.Contains(string(data), reference.SourceSymbol) {
			return fmt.Errorf("status stream source symbol %q is missing from %s", reference.SourceSymbol, reference.SourceFile)
		}
	}

	const (
		streamSource    = "applications/services/web_server/http_api/api_status_streaming.c"
		rootSource      = "applications/services/web_server/http_api/api_root.c"
		publisherSource = "applications/services/state_publisher/state_publisher.c"
	)
	streamData, err := readFirmwareFile(root, streamSource, checked)
	if err != nil {
		return err
	}
	rootData, err := readFirmwareFile(root, rootSource, checked)
	if err != nil {
		return err
	}
	publisherData, err := readFirmwareFile(root, publisherSource, checked)
	if err != nil {
		return err
	}

	if err := checkDefine(streamSource, streamData, "MAX_CLIENTS_COUNT", contract.MaxClients); err != nil {
		return err
	}
	if err := checkDefine(streamSource, streamData, "FRAME_INTERVAL_MS", contract.FrameIntervalMS); err != nil {
		return err
	}
	if err := checkDefine(streamSource, streamData, "CLIENT_HEARTBEAT_INTERVAL_MS", contract.ClientHeartbeatIntervalMS); err != nil {
		return err
	}
	if err := checkDefine(publisherSource, publisherData, "HEARTBEAT_INTERVAL_MS", contract.PublisherHeartbeatMS); err != nil {
		return err
	}

	rateLimit := rateLimitPattern.FindSubmatch(streamData)
	if rateLimit == nil || string(rateLimit[1]) != fmt.Sprint(contract.RateLimitMaxPackets) || string(rateLimit[2]) != fmt.Sprint(contract.RateLimitPeriodMS) {
		return fmt.Errorf("%s rate limit does not match receipt (%d packets per %d ms)", streamSource, contract.RateLimitMaxPackets, contract.RateLimitPeriodMS)
	}

	for _, required := range []struct {
		file string
		data []byte
		text string
	}{
		{streamSource, streamData, `"$.enable"`},
		{streamSource, streamData, `"$.send"`},
		{streamSource, streamData, `strcmp("all", send_value)`},
		{streamSource, streamData, "BSB_Error_Severity_FATAL"},
		{streamSource, streamData, "BSB_Error_Cause_" + contract.FatalErrorCause},
		{rootSource, rootData, `"` + contract.AccessKeyQuery + `"`},
		{rootSource, rootData, `"` + contract.APISemVerQuery + `"`},
		{publisherSource, publisherData, "state_publisher_send_complete_snapshot"},
		{publisherSource, publisherData, "state_publisher_collect_all"},
		{publisherSource, publisherData, "screen_streamer_front"},
		{publisherSource, publisherData, "GuiDisplayIdFront"},
	} {
		if !strings.Contains(string(required.data), required.text) {
			return fmt.Errorf("%s is missing %q", required.file, required.text)
		}
	}
	if contract.FrontFramesOnly && strings.Contains(string(publisherData), "screen_streamer_back") {
		return fmt.Errorf("%s now contains a back-display streamer; refresh the status stream receipt", publisherSource)
	}
	return nil
}

func checkFrames(root string, contract internalapi.FrameContract, checked map[string][]byte) error {
	for _, reference := range contract.SourceReferences {
		data, err := readFirmwareFile(root, reference.SourceFile, checked)
		if err != nil {
			return fmt.Errorf("frames: %w", err)
		}
		if !strings.Contains(string(data), reference.SourceSymbol) {
			return fmt.Errorf("frame source symbol %q is missing from %s", reference.SourceSymbol, reference.SourceFile)
		}
	}

	const (
		httpSource          = "applications/services/web_server/http_api/api_streaming.c"
		streamerSource      = "applications/services/state_publisher/screen_streamer.c"
		subscriptionsSource = "applications/services/state_publisher/subscriptions.c"
		colorSource         = "lib/toolbox/color.c"
		frontSource         = "applications/services/front_display/front_display.h"
		backSource          = "applications/services/back_display/back_display.h"
		canvasSource        = "applications/services/gui/modules/canvas.c"
	)
	httpData, err := readFirmwareFile(root, httpSource, checked)
	if err != nil {
		return err
	}
	streamerData, err := readFirmwareFile(root, streamerSource, checked)
	if err != nil {
		return err
	}
	subscriptionsData, err := readFirmwareFile(root, subscriptionsSource, checked)
	if err != nil {
		return err
	}
	colorData, err := readFirmwareFile(root, colorSource, checked)
	if err != nil {
		return err
	}
	frontData, err := readFirmwareFile(root, frontSource, checked)
	if err != nil {
		return err
	}
	backData, err := readFirmwareFile(root, backSource, checked)
	if err != nil {
		return err
	}
	canvasData, err := readFirmwareFile(root, canvasSource, checked)
	if err != nil {
		return err
	}

	for _, check := range []struct {
		file string
		data []byte
		name string
		want int
	}{
		{frontSource, frontData, "FRONT_DISPLAY_W", contract.Front.Width},
		{frontSource, frontData, "FRONT_DISPLAY_H", contract.Front.Height},
		{frontSource, frontData, "FRONT_DISPLAY_BPP", 24},
		{backSource, backData, "BACK_DISPLAY_W", contract.Back.Width},
		{backSource, backData, "BACK_DISPLAY_H", contract.Back.Height},
		{backSource, backData, "BACK_DISPLAY_BPP", 8},
	} {
		if err := checkDefine(check.file, check.data, check.name, check.want); err != nil {
			return err
		}
	}
	if contract.Front.PlainBytes != contract.Front.Width*contract.Front.Height*3 {
		return fmt.Errorf("front frame plain size = %d, dimensions require %d", contract.Front.PlainBytes, contract.Front.Width*contract.Front.Height*3)
	}
	if contract.Back.PlainBytes != contract.Back.Width*contract.Back.Height/2 {
		return fmt.Errorf("back frame plain size = %d, dimensions require %d", contract.Back.PlainBytes, contract.Back.Width*contract.Back.Height/2)
	}

	for _, required := range []struct {
		file string
		data []byte
		text string
	}{
		{httpSource, httpData, "FRONT_DISPLAY_BUF_SIZE"},
		{httpSource, httpData, "BACK_DISPLAY_BUF_SIZE"},
		{httpSource, httpData, "color_buf_l8_to_l4"},
		{streamerSource, streamerData, "ScreenStreamerPixelFormatR8G8B8"},
		{streamerSource, streamerData, "ScreenStreamerPixelFormatL4"},
		{streamerSource, streamerData, "const uint8_t blk_size = instance->display_id == GuiDisplayIdFront ? 3 : 2;"},
		{streamerSource, streamerData, "ScreenStreamerCompressionRLE"},
		{streamerSource, streamerData, "ScreenStreamerCompressionPlain"},
		{subscriptionsSource, subscriptionsData, "ScreenStreamerCompressionPlain] = BSB_Frame_Encoding_PLAIN"},
		{subscriptionsSource, subscriptionsData, "ScreenStreamerCompressionRLE] = BSB_Frame_Encoding_RUN_LENGTH"},
		{subscriptionsSource, subscriptionsData, "ScreenStreamerPixelFormatR8G8B8] = BSB_Frame_PixelFormat_RGB888"},
		{subscriptionsSource, subscriptionsData, "ScreenStreamerPixelFormatL8] = BSB_Frame_PixelFormat_L8"},
		{subscriptionsSource, subscriptionsData, "ScreenStreamerPixelFormatL4] = BSB_Frame_PixelFormat_L4"},
		{subscriptionsSource, subscriptionsData, "GuiDisplayIdFront] = BSB_Frame_Screen_FRONT"},
		{subscriptionsSource, subscriptionsData, "GuiDisplayIdBack] = BSB_Frame_Screen_BACK"},
		{colorSource, colorData, "(src_u8[src_i] >> 4) | (src_u8[src_i + 1] & 0xF0)"},
		{canvasSource, canvasData, "data[2] = color.red;"},
		{canvasSource, canvasData, "data[1] = color.green;"},
		{canvasSource, canvasData, "data[0] = color.blue;"},
	} {
		if !strings.Contains(string(required.data), required.text) {
			return fmt.Errorf("%s is missing %q", required.file, required.text)
		}
	}
	return nil
}

func checkSnapshots(root string, contract internalapi.Contract, checked map[string][]byte) error {
	for _, reference := range contract.Snapshots.SourceReferences {
		data, err := readFirmwareFile(root, reference.SourceFile, checked)
		if err != nil {
			return fmt.Errorf("snapshots: %w", err)
		}
		if !strings.Contains(string(data), reference.SourceSymbol) {
			return fmt.Errorf("snapshot source symbol %q is missing from %s", reference.SourceSymbol, reference.SourceFile)
		}
	}

	for _, endpoint := range contract.Snapshots.HTTP {
		operation, ok := contract.Operation("GET " + endpoint.Path)
		if !ok || operation.Phase != 3 {
			return fmt.Errorf("snapshot endpoint GET %s is not owned by phase 3", endpoint.Path)
		}
		data, err := readFirmwareFile(root, operation.SourceFile, checked)
		if err != nil {
			return err
		}
		for _, key := range endpoint.CanonicalKeys {
			if !containsJSONKey(data, key) {
				return fmt.Errorf("%s is missing canonical snapshot key %q", operation.SourceFile, key)
			}
		}
	}

	const subscriptionsSource = "applications/services/state_publisher/subscriptions.c"
	subscriptions, err := readFirmwareFile(root, subscriptionsSource, checked)
	if err != nil {
		return err
	}
	for _, kind := range contract.Snapshots.StateUpdateKinds {
		marker := "BSB_State_StateUpdate_" + kind + "_tag"
		if !strings.Contains(string(subscriptions), marker) {
			return fmt.Errorf("%s is missing %q", subscriptionsSource, marker)
		}
	}
	return nil
}

func checkOptionalTools(root string, contract internalapi.OptionalToolsContract, checked map[string][]byte) error {
	for _, reference := range append(contract.CLI.SourceReferences, contract.Media.SourceReferences...) {
		data, err := readFirmwareFile(root, reference.SourceFile, checked)
		if err != nil {
			return fmt.Errorf("optional tools: %w", err)
		}
		if !strings.Contains(string(data), reference.SourceSymbol) {
			return fmt.Errorf("optional tools source symbol %q is missing from %s", reference.SourceSymbol, reference.SourceFile)
		}
	}

	for _, command := range contract.CLI.Commands {
		data, err := readFirmwareFile(root, command.SourceFile, checked)
		if err != nil {
			return fmt.Errorf("CLI command %s: %w", command.Name, err)
		}
		if !strings.Contains(string(data), `name="`+command.Name+`"`) {
			return fmt.Errorf("CLI command %q is missing from %s", command.Name, command.SourceFile)
		}
		if !strings.Contains(string(data), command.SourceSymbol) {
			return fmt.Errorf("CLI command %q source symbol %q is missing from %s", command.Name, command.SourceSymbol, command.SourceFile)
		}
	}

	const (
		cliSocketSource  = "applications/services/cli_socket/cli_socket.c"
		usbNetworkSource = "applications/services/usb_network/settings/usb_network_settings_interface_v1.c"
		promptSource     = "lib/cli/shell/cli_shell_line.c"
		powerSource      = "applications/services/power/power_cli.c"
		pngSource        = "targets/f21/config/lv_conf.h"
		displaySource    = "applications/services/web_server/http_api/api_display.c"
		audioSource      = "applications/services/audio/audio.c"
		audioHeader      = "applications/services/audio/audio.h"
	)
	cliSocket, err := readFirmwareFile(root, cliSocketSource, checked)
	if err != nil {
		return err
	}
	if err := checkDefine(cliSocketSource, cliSocket, "CLI_SOCKET_PORT", contract.CLI.Port); err != nil {
		return err
	}
	usbNetwork, err := readFirmwareFile(root, usbNetworkSource, checked)
	if err != nil {
		return err
	}
	addressMarker := strings.ReplaceAll(contract.CLI.DefaultAddress, ".", ", ")
	if !strings.Contains(string(usbNetwork), "{"+addressMarker+"}") {
		return fmt.Errorf("%s is missing USB network address %s", usbNetworkSource, contract.CLI.DefaultAddress)
	}
	prompt, err := readFirmwareFile(root, promptSource, checked)
	if err != nil {
		return err
	}
	if !strings.Contains(string(prompt), `"%s`+contract.CLI.Prompt+`"`) {
		return fmt.Errorf("%s is missing CLI prompt %q", promptSource, contract.CLI.Prompt)
	}
	power, err := readFirmwareFile(root, powerSource, checked)
	if err != nil {
		return err
	}
	for _, marker := range []string{"power_cli_reboot", `"sw"`} {
		if !strings.Contains(string(power), marker) {
			return fmt.Errorf("%s is missing %q", powerSource, marker)
		}
	}

	png, err := readFirmwareFile(root, pngSource, checked)
	if err != nil {
		return err
	}
	if err := checkDefine(pngSource, png, "LV_USE_LODEPNG", 1); err != nil {
		return err
	}
	display, err := readFirmwareFile(root, displaySource, checked)
	if err != nil {
		return err
	}
	for _, marker := range []string{"header.w > display_parameters->width", "header.h > display_parameters->height"} {
		if !strings.Contains(string(display), marker) {
			return fmt.Errorf("%s is missing %q", displaySource, marker)
		}
	}
	audio, err := readFirmwareFile(root, audioSource, checked)
	if err != nil {
		return err
	}
	if err := checkDefine(audioSource, audio, "AUDIO_SAMPLE_RATE", contract.Media.Audio.SampleRateHz); err != nil {
		return err
	}
	for _, marker := range []string{"int16_t buffer", "storage_file_read"} {
		if !strings.Contains(string(audio), marker) {
			return fmt.Errorf("%s is missing %q", audioSource, marker)
		}
	}
	header, err := readFirmwareFile(root, audioHeader, checked)
	if err != nil {
		return err
	}
	for _, marker := range []string{"Header: none", "Channels: 1", "Rate: 44100 Hz", "Bits: 16bit LE"} {
		if !strings.Contains(string(header), marker) {
			return fmt.Errorf("%s is missing %q", audioHeader, marker)
		}
	}
	return nil
}

func checkRemoteMQTT(root string, contract internalapi.RemoteContract, checked map[string][]byte) error {
	for _, reference := range contract.SourceReferences {
		data, err := readFirmwareFile(root, reference.SourceFile, checked)
		if err != nil {
			return fmt.Errorf("remote MQTT: %w", err)
		}
		if !strings.Contains(string(data), reference.SourceSymbol) {
			return fmt.Errorf("remote MQTT source symbol %q is missing from %s", reference.SourceSymbol, reference.SourceFile)
		}
	}

	const (
		connectionSource   = "applications/services/mqtt/mqtt_connection.c"
		internalSource     = "applications/services/mqtt/mqtt_i.h"
		subscriptionSource = "applications/services/mqtt/mqtt_subscription.c"
		messageSource      = "applications/services/mqtt/mqtt_message.c"
		httpSource         = "applications/services/mqtt/modules/mqtt_http_proxy.c"
		streamSource       = "applications/services/mqtt/modules/mqtt_streaming.c"
	)
	connection, err := readFirmwareFile(root, connectionSource, checked)
	if err != nil {
		return err
	}
	if err := checkDefine(connectionSource, connection, "MQTT_VERSION", contract.MQTTVersion); err != nil {
		return err
	}
	internal, err := readFirmwareFile(root, internalSource, checked)
	if err != nil {
		return err
	}
	for name, want := range map[string]string{
		"MQTT_API_VERSION":        contract.APIVersion,
		"MQTT_ROOT_TOPIC_SESSION": "sessions",
		"MQTT_DIRECTION_UP":       contract.UpDirection,
		"MQTT_DIRECTION_DOWN":     contract.DownDirection,
	} {
		if err := checkStringDefine(internalSource, internal, name, want); err != nil {
			return err
		}
	}

	subscription, err := readFirmwareFile(root, subscriptionSource, checked)
	if err != nil {
		return err
	}
	for _, marker := range []string{
		`root = MQTT_ROOT_TOPIC_SESSION;`,
		`furi_string_printf(out, "%s/%s/%s/%s/%s", root, id, dir, MQTT_API_VERSION, topic);`,
		"MQTT_DIRECTION_DOWN",
		"MQTT_DIRECTION_UP",
	} {
		if !strings.Contains(string(subscription), marker) {
			return fmt.Errorf("%s is missing %q", subscriptionSource, marker)
		}
	}
	message, err := readFirmwareFile(root, messageSource, checked)
	if err != nil {
		return err
	}
	responseTopicMarker := `MQTT_ROOT_TOPIC_SESSION "/*/" MQTT_DIRECTION_UP "/" MQTT_API_VERSION "/#"`
	if !strings.Contains(string(message), responseTopicMarker) {
		return fmt.Errorf("%s is missing %q", messageSource, responseTopicMarker)
	}

	httpData, err := readFirmwareFile(root, httpSource, checked)
	if err != nil {
		return err
	}
	for name, want := range map[string]string{
		"HTTP_HOST":           contract.HTTP.LocalHost,
		"HTTP_URI_API_PREFIX": contract.HTTP.PathPrefix,
		"SUB_TOPIC":           contract.HTTP.RequestTopic,
	} {
		if err := checkStringDefine(httpSource, httpData, name, want); err != nil {
			return err
		}
	}
	for _, marker := range []string{
		`#define SUB_QOS (MqttQosExactlyOnce)`,
		`[MqttHttpProxyMethodIdGet] = "GET"`,
		`[MqttHttpProxyMethodIdPost] = "POST"`,
		`[MqttHttpProxyMethodIdPut] = "PUT"`,
		`[MqttHttpProxyMethodIdDelete] = "DELETE"`,
		"mqtt_http_proxy_request_requires_response",
		"MqttPropertyTypeResponseTopic",
		"MqttPropertyTypeCorrelationData",
		"mqtt_publish_ex",
		"MqttQosAtLeastOnce",
		fmt.Sprintf("HTTP/1.1 %d Unprocessable Entity", contract.HTTP.InvalidStatus),
		"mqtt_http_proxy_is_websocket_upgrade",
	} {
		if !strings.Contains(string(httpData), marker) {
			return fmt.Errorf("%s is missing %q", httpSource, marker)
		}
	}
	if err := checkDefine(httpSource, httpData, "HTTP_CONN_TIMEOUT_MS", contract.HTTP.TimeoutMS); err != nil {
		return err
	}
	wantBlocklist := make([]string, 0, len(contract.HTTP.BlockedOperations))
	methodIDs := map[string]string{
		"GET":    "MqttHttpProxyMethodIdGet",
		"POST":   "MqttHttpProxyMethodIdPost",
		"PUT":    "MqttHttpProxyMethodIdPut",
		"DELETE": "MqttHttpProxyMethodIdDelete",
	}
	for _, operation := range contract.HTTP.BlockedOperations {
		parts := strings.SplitN(operation, " ", 2)
		if len(parts) != 2 || !strings.HasPrefix(parts[1], "/api/") {
			return fmt.Errorf("invalid remote blocklist receipt entry %q", operation)
		}
		wantBlocklist = append(wantBlocklist, strings.TrimPrefix(parts[1], "/api/")+" "+methodIDs[parts[0]])
	}
	gotMatches := blocklistPattern.FindAllSubmatch(httpData, -1)
	gotBlocklist := make([]string, 0, len(gotMatches))
	for _, match := range gotMatches {
		gotBlocklist = append(gotBlocklist, string(match[1])+" "+string(match[2]))
	}
	if !slices.Equal(gotBlocklist, wantBlocklist) {
		return fmt.Errorf("%s blocklist = %v, receipt = %v", httpSource, gotBlocklist, wantBlocklist)
	}

	streamData, err := readFirmwareFile(root, streamSource, checked)
	if err != nil {
		return err
	}
	if err := checkStringDefine(streamSource, streamData, "SUB_TOPIC", contract.Stream.RequestTopic); err != nil {
		return err
	}
	for _, marker := range []string{
		`#define SUB_QOS (MqttQosAtLeastOnce)`,
		`#define PUB_QOS (MqttQosAtMostOnce)`,
		"MqttPropertyTypeExpiryInterval",
		"MqttPropertyTypeResponseTopic",
		`.type = data_size ? MqttStreamingApiMessageTypeStart : MqttStreamingApiMessageTypeStop`,
		`"message_limits"`,
		`"` + contract.Stream.MessageLimitMaxCountKey + `"`,
		`"` + contract.Stream.MessageLimitIntervalSecondsKey + `"`,
		"state_publisher_handle",
		"response_topic",
		"state_publisher_add_transport",
	} {
		if !strings.Contains(string(streamData), marker) {
			return fmt.Errorf("%s is missing %q", streamSource, marker)
		}
	}
	for name, want := range map[string]int{
		"API_QUEUE_SIZE":            contract.Stream.QueueSize,
		"FRAME_PERIOD_MS":           contract.Stream.FrameIntervalMS,
		"EXPIRY_INTERVAL_DEFAULT_S": contract.Stream.DefaultExpirySeconds,
	} {
		if err := checkDefine(streamSource, streamData, name, want); err != nil {
			return err
		}
	}
	if contract.Stream.SnapshotOnStart || strings.Contains(string(streamData), "state_publisher_send_complete_snapshot") {
		return fmt.Errorf("%s remote stream now requests a complete snapshot", streamSource)
	}
	const publisherSource = "applications/services/state_publisher/state_publisher.c"
	publisherData, err := readFirmwareFile(root, publisherSource, checked)
	if err != nil {
		return err
	}
	addTransportBody, err := cFunctionBody(publisherSource, publisherData, "state_publisher_add_transport")
	if err != nil {
		return err
	}
	if strings.Contains(string(addTransportBody), "state_publisher_send_complete_snapshot") {
		return fmt.Errorf("%s state_publisher_add_transport now requests a complete snapshot", publisherSource)
	}
	return nil
}

func containsJSONKey(data []byte, key string) bool {
	source := string(data)
	return strings.Contains(source, `"`+key+`"`) ||
		strings.Contains(source, `\"`+key+`\"`)
}

func readFirmwareFile(root, sourceFile string, checked map[string][]byte) ([]byte, error) {
	if data, ok := checked[sourceFile]; ok {
		return data, nil
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(sourceFile)))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", sourceFile, err)
	}
	checked[sourceFile] = data
	return data, nil
}

func checkDefine(sourceFile string, data []byte, name string, want int) error {
	for _, match := range defineIntPattern.FindAllSubmatch(data, -1) {
		if string(match[1]) == name {
			if string(match[2]) != fmt.Sprint(want) {
				return fmt.Errorf("%s %s = %s, receipt = %d", sourceFile, name, match[2], want)
			}
			return nil
		}
	}
	return fmt.Errorf("%s is missing integer define %s", sourceFile, name)
}

func checkStringDefine(sourceFile string, data []byte, name, want string) error {
	pattern := regexp.MustCompile(`#define\s+` + regexp.QuoteMeta(name) + `\s+"([^"]*)"`)
	match := pattern.FindSubmatch(data)
	if match == nil {
		return fmt.Errorf("%s is missing string define %s", sourceFile, name)
	}
	if string(match[1]) != want {
		return fmt.Errorf("%s %s = %q, receipt = %q", sourceFile, name, match[1], want)
	}
	return nil
}

func cFunctionBody(sourceFile string, data []byte, symbol string) ([]byte, error) {
	pattern := regexp.MustCompile(`(?s)\b` + regexp.QuoteMeta(symbol) + `\s*\([^;{]*\)\s*\{`)
	match := pattern.FindIndex(data)
	if match == nil {
		return nil, fmt.Errorf("%s is missing function definition %s", sourceFile, symbol)
	}
	open := match[1] - 1
	depth := 0
	for index := open; index < len(data); index++ {
		switch data[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return data[open+1 : index], nil
			}
		}
	}
	return nil, fmt.Errorf("%s function definition %s has no closing brace", sourceFile, symbol)
}

func gitOutput(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output)), nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
