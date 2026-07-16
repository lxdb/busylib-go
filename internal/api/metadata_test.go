package api

import (
	"reflect"
	"testing"
)

func TestLocalOnlyOperationsRemainAnExplicitProxyPolicy(t *testing.T) {
	expected := []string{
		"DELETE /api/account",
		"PUT /api/account/backend",
		"POST /api/account/link",
		"POST /api/wifi/connect",
		"POST /api/wifi/disconnect",
		"GET /api/wifi/networks",
	}
	if !reflect.DeepEqual(LocalOnlyOperations(), expected) {
		t.Fatalf("local-only operations = %#v, want %#v", LocalOnlyOperations(), expected)
	}
}

func TestIsLocalOnlyOperation(t *testing.T) {
	if !IsLocalOnlyOperation("post", "/api/wifi/connect") {
		t.Fatalf("POST /api/wifi/connect should be local-only")
	}
	if IsLocalOnlyOperation("GET", "/api/wifi/status") {
		t.Fatalf("GET /api/wifi/status should not be local-only")
	}
}
