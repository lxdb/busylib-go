package api

import (
	"reflect"
	"sort"
	"testing"
)

func TestRemoteBlockedOperationsMatchFirmwareMQTTProxy(t *testing.T) {
	contract, err := LoadContractFile("testdata/firmware-contract.json")
	if err != nil {
		t.Fatalf("load firmware contract: %v", err)
	}
	want := append([]string(nil), contract.Remote.HTTP.BlockedOperations...)
	got := RemoteBlockedOperations()
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("remote-blocked operations = %#v, receipt = %#v", got, want)
	}
}

func TestIsRemoteBlockedOperation(t *testing.T) {
	if !IsRemoteBlockedOperation("post", "/api/update") {
		t.Fatalf("POST /api/update should be blocked remotely")
	}
	if !IsRemoteBlockedOperation("post", "/api/update/") {
		t.Fatalf("POST /api/update/ should match the firmware's trailing-slash blocklist behavior")
	}
	if IsRemoteBlockedOperation("GET", "/api/wifi/status") {
		t.Fatalf("GET /api/wifi/status should be available remotely")
	}
}
