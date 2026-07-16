package api

import (
	"os"
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

	if contract.FirmwareCommit != "1add7be4f1fd31cbd0763c4c20add1ff6382232e" {
		t.Fatalf("firmware commit = %q", contract.FirmwareCommit)
	}
	if contract.ProtobufCommit != "07223321a4ab39a13c5167dbf85c87c418325634" {
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

func TestFirmwareContractFramesMatchDecoderAndSelectedProtobuf(t *testing.T) {
	contract, err := LoadContractFile("testdata/firmware-contract.json")
	if err != nil {
		t.Fatalf("load firmware contract: %v", err)
	}

	if contract.Frames.HTTPPath != "/api/screen" || contract.Frames.MaxPayloadBytes != framepkg.MaxPayloadSize {
		t.Fatalf("frame path/max payload = %q/%d", contract.Frames.HTTPPath, contract.Frames.MaxPayloadBytes)
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

	options, err := os.ReadFile("../protosrc/bsb-protobuf/frame.options")
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
