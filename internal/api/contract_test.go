package api

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	framepkg "github.com/lxdb/busylib-go/frame"
	"github.com/lxdb/busylib-go/proto/framepb"
	"github.com/lxdb/busylib-go/proto/statepb"
	publicstream "github.com/lxdb/busylib-go/stream"
)

func TestFirmwareContractReceipt(t *testing.T) {
	contract, err := LoadContractFile("testdata/firmware-contract.json")
	if err != nil {
		t.Fatalf("load firmware contract: %v", err)
	}

	if contract.FirmwareCommit != "ac59f45cfcd14f6b0fccb8e8e8f47e183a537aaf" {
		t.Fatalf("firmware commit = %q", contract.FirmwareCommit)
	}
	if contract.ProtobufCommit != "dba670e2ddb5cda511af997ca5fcb1254e90917f" {
		t.Fatalf("protobuf commit = %q", contract.ProtobufCommit)
	}
	if contract.License != "GPL-2.0-or-later" {
		t.Fatalf("firmware license = %q", contract.License)
	}
}

func TestFirmwareContractOwnsStatusWebSocketInPhaseSix(t *testing.T) {
	contract, err := LoadContractFile("testdata/firmware-contract.json")
	if err != nil {
		t.Fatalf("load firmware contract: %v", err)
	}

	operation, ok := contract.Operation("GET /api/status/ws")
	if !ok {
		t.Fatal("GET /api/status/ws is missing")
	}
	if operation.Phase != StreamPhase {
		t.Fatalf("GET /api/status/ws phase = %d, want %d", operation.Phase, StreamPhase)
	}
}

func TestFirmwareContractStatusStreamMatchesGeneratedProtobuf(t *testing.T) {
	contract, err := LoadContractFile("testdata/firmware-contract.json")
	if err != nil {
		t.Fatalf("load firmware contract: %v", err)
	}

	stateUpdate := statepb.File_state_proto.Messages().ByName("StateUpdate")
	if stateUpdate == nil || stateUpdate.Oneofs().Len() != 1 {
		t.Fatal("generated StateUpdate oneof is missing")
	}
	got := stateUpdate.Oneofs().Get(0).Fields().Len()
	if got != contract.StatusStream.StateUpdateKinds {
		t.Fatalf("generated update kinds = %d, receipt = %d", got, contract.StatusStream.StateUpdateKinds)
	}
}

func TestFirmwareContractFramesMatchDecoderAndGeneratedProtobuf(t *testing.T) {
	contract, err := LoadContractFile("testdata/firmware-contract.json")
	if err != nil {
		t.Fatalf("load firmware contract: %v", err)
	}

	if contract.Frames.HTTPPath != "/api/screen" || contract.Frames.MaxPayloadBytes != framepkg.MaxPayloadSize {
		t.Fatalf("frame path/max payload = %q/%d", contract.Frames.HTTPPath, contract.Frames.MaxPayloadBytes)
	}
	if contract.Frames.HTTPEncoding != "base64" {
		t.Fatalf("frame HTTP encoding = %q, want base64", contract.Frames.HTTPEncoding)
	}
	if contract.Frames.Front.Screen != int(framepb.Screen_FRONT) ||
		contract.Frames.Front.Width != framepkg.FrontWidth ||
		contract.Frames.Front.Height != framepkg.FrontHeight {
		t.Fatalf("front frame receipt = %#v", contract.Frames.Front)
	}
	if contract.Frames.Back.Screen != int(framepb.Screen_BACK) ||
		contract.Frames.Back.Width != framepkg.BackWidth ||
		contract.Frames.Back.Height != framepkg.BackHeight {
		t.Fatalf("back frame receipt = %#v", contract.Frames.Back)
	}

	operation, ok := contract.Operation("GET /api/screen")
	if !ok || operation.Phase != 3 {
		t.Fatalf("GET /api/screen operation = %#v, present = %v", operation, ok)
	}
	if len(framepb.Encoding_name) != 4 || len(framepb.PixelFormat_name) != len(contract.Frames.ProtobufPixelFormats) {
		t.Fatalf("generated frame enums = %d encodings/%d formats", len(framepb.Encoding_name), len(framepb.PixelFormat_name))
	}

	protoSource := os.Getenv("BUSYLIB_GO_PROTO_SRC")
	if protoSource == "" {
		t.Skip("BUSYLIB_GO_PROTO_SRC is required to verify frame.options")
	}
	options, err := os.ReadFile(filepath.Join(protoSource, "frame.options"))
	if err != nil {
		t.Fatalf("read selected frame options: %v", err)
	}
	if !strings.Contains(string(options), "BSB_Frame.Frame.data    max_size:16384") {
		t.Fatalf("selected frame options do not contain the %d-byte limit", contract.Frames.MaxPayloadBytes)
	}
}

func TestFirmwareContractSnapshotsMatchPublicStreamKindsAndKeepOperationOwnership(t *testing.T) {
	contract, err := LoadContractFile("testdata/firmware-contract.json")
	if err != nil {
		t.Fatalf("load firmware contract: %v", err)
	}

	wantHTTP := []SnapshotHTTPContract{
		{Section: "name", Path: "/api/name", CanonicalKeys: []string{"name"}},
		{Section: "version", Path: "/api/version", CanonicalKeys: []string{"api_semver"}},
		{Section: "status", Path: "/api/status", CanonicalKeys: []string{"device", "firmware", "system", "power"}},
		{Section: "system", Path: "/api/status/system", CanonicalKeys: []string{"api_semver", "uptime", "boot_time", "auto_update_enabled"}},
		{Section: "power", Path: "/api/status/power", CanonicalKeys: []string{"state", "battery_charge", "battery_voltage", "battery_current", "usb_voltage"}},
		{Section: "time", Path: "/api/time", CanonicalKeys: []string{"timestamp"}},
		{Section: "wifi", Path: "/api/wifi/status", CanonicalKeys: []string{"state", "ssid", "bssid", "channel", "rssi", "security", "ip_config"}},
		{Section: "brightness", Path: "/api/display/brightness", CanonicalKeys: []string{"value"}},
		{Section: "audio_volume", Path: "/api/audio/volume", CanonicalKeys: []string{"volume"}},
		{Section: "ble", Path: "/api/ble/status", CanonicalKeys: []string{"status", "address"}},
		{Section: "storage", Path: "/api/storage/status", CanonicalKeys: []string{"used_bytes", "free_bytes", "total_bytes"}},
	}
	if !reflect.DeepEqual(contract.Snapshots.HTTP, wantHTTP) {
		t.Fatalf("snapshot HTTP receipt = %#v", contract.Snapshots.HTTP)
	}

	wantKinds := []string{
		string(publicstream.UpdateDeviceName),
		string(publicstream.UpdatePower),
		string(publicstream.UpdateBrightness),
		string(publicstream.UpdateAudioVolume),
		string(publicstream.UpdateWiFi),
		string(publicstream.UpdateFirmware),
		string(publicstream.UpdateCheck),
		string(publicstream.UpdateTimezone),
		string(publicstream.UpdateMatter),
		string(publicstream.UpdateFrame),
		string(publicstream.UpdateInput),
		string(publicstream.UpdateTimer),
		string(publicstream.UpdateBLE),
		string(publicstream.UpdateAutoUpdate),
		string(publicstream.UpdateTimerProfiles),
	}
	if !reflect.DeepEqual(contract.Snapshots.StateUpdateKinds, wantKinds) {
		t.Fatalf("snapshot stream kinds = %#v, want %#v", contract.Snapshots.StateUpdateKinds, wantKinds)
	}
	for _, endpoint := range contract.Snapshots.HTTP {
		operation, ok := contract.Operation("GET " + endpoint.Path)
		if !ok || operation.Phase != 3 {
			t.Fatalf("snapshot operation GET %s = %#v, present=%v", endpoint.Path, operation, ok)
		}
	}
	operation, ok := contract.Operation("GET /api/status/ws")
	if !ok || operation.Phase != StreamPhase {
		t.Fatalf("stream operation = %#v, present=%v", operation, ok)
	}
}

func TestFirmwareContractOptionalToolsMatchPhaseNineSurface(t *testing.T) {
	contract, err := LoadContractFile("testdata/firmware-contract.json")
	if err != nil {
		t.Fatalf("load firmware contract: %v", err)
	}

	cli := contract.OptionalTools.CLI
	if cli.DefaultAddress != "10.0.4.20" || cli.Port != 23 || cli.Prompt != ">: " || cli.InterruptByte != 3 {
		t.Fatalf("CLI transport receipt = %#v", cli)
	}
	if cli.RebootCommand != "power reboot sw" {
		t.Fatalf("CLI reboot command = %q", cli.RebootCommand)
	}
	wantCommands := []string{
		"uptime", "power", "storage", "update", "input", "loader", "top", "free",
		"free_blocks", "log", "echo", "device_info", "date", "timezone", "matter",
		"audio", "display", "sysctl", "log_dump",
	}
	gotCommands := make([]string, 0, len(cli.Commands))
	for _, command := range cli.Commands {
		gotCommands = append(gotCommands, command.Name)
	}
	if !reflect.DeepEqual(gotCommands, wantCommands) {
		t.Fatalf("CLI commands = %#v, want %#v", gotCommands, wantCommands)
	}

	media := contract.OptionalTools.Media
	if media.Image.OutputFormat != "PNG" || media.Image.Decoder != "LODEPNG" ||
		media.Image.FrontMaxWidth != framepkg.FrontWidth || media.Image.FrontMaxHeight != framepkg.FrontHeight ||
		media.Image.BackMaxWidth != framepkg.BackWidth || media.Image.BackMaxHeight != framepkg.BackHeight {
		t.Fatalf("image conversion receipt = %#v", media.Image)
	}
	if media.Audio.Header != "none" || media.Audio.Channels != 1 || media.Audio.SampleRateHz != 44_100 ||
		media.Audio.BitsPerSample != 16 || media.Audio.ByteOrder != "little_endian" || media.Audio.OutputExtension != ".snd" {
		t.Fatalf("audio conversion receipt = %#v", media.Audio)
	}
}

func TestFirmwareContractRemoteMQTTMatchesPhaseTenSurface(t *testing.T) {
	contract, err := LoadContractFile("testdata/firmware-contract.json")
	if err != nil {
		t.Fatalf("load firmware contract: %v", err)
	}

	remote := contract.Remote
	if remote.MQTTVersion != 5 || remote.APIVersion != "v1" || remote.TopicPattern != "sessions/{session_id}/{direction}/v1/{topic}" {
		t.Fatalf("remote MQTT routing receipt = %#v", remote)
	}
	if remote.DownDirection != "down" || remote.UpDirection != "up" {
		t.Fatalf("remote MQTT directions = %q/%q", remote.DownDirection, remote.UpDirection)
	}

	wantBlocked := []string{
		"POST /api/update",
		"DELETE /api/account",
		"POST /api/account/link",
		"PUT /api/account/backend",
		"POST /api/wifi/connect",
		"POST /api/wifi/disconnect",
		"GET /api/wifi/networks",
	}
	if remote.HTTP.RequestTopic != "http-request" || remote.HTTP.LocalHost != "http://127.0.0.1" ||
		remote.HTTP.PathPrefix != "/api/" || remote.HTTP.TimeoutMS != 5_000 ||
		remote.HTTP.RequestQoS != 2 || remote.HTTP.ResponseQoS != 1 || remote.HTTP.InvalidStatus != 422 ||
		!remote.HTTP.RequiresResponseTopic || !remote.HTTP.RequiresCorrelationData || !remote.HTTP.EchoesCorrelationData ||
		!reflect.DeepEqual(remote.HTTP.BlockedOperations, wantBlocked) {
		t.Fatalf("remote HTTP receipt = %#v", remote.HTTP)
	}

	if remote.Stream.RequestTopic != "stream-request" || remote.Stream.RequestQoS != 1 || remote.Stream.ResponseQoS != 0 ||
		remote.Stream.DefaultExpirySeconds != 60 || remote.Stream.FrameIntervalMS != 500 || remote.Stream.QueueSize != 4 ||
		!remote.Stream.EmptyPayloadStops || !remote.Stream.NonEmptyPayloadStarts || remote.Stream.SnapshotOnStart ||
		!remote.Stream.SinglePublisher || remote.Stream.MessageLimitMaxCountKey != "max_count" ||
		remote.Stream.MessageLimitIntervalSecondsKey != "interval_s" {
		t.Fatalf("remote stream receipt = %#v", remote.Stream)
	}
}
