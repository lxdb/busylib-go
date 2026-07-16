package api

import (
	"os"
	"strings"
	"testing"

	framepkg "github.com/lxdb/busylib-go/frame"
	"github.com/lxdb/busylib-go/proto/framepb"
	"github.com/lxdb/busylib-go/proto/statepb"
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
