package api

import "testing"

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
