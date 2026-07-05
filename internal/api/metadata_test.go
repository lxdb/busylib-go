package api

import (
	"reflect"
	"testing"
)

func TestLocalOnlyOperationsMatchOpenAPIInventory(t *testing.T) {
	inventory := readOpenAPI(t)

	var fromInventory []string
	for _, operation := range inventory.Operations {
		if operation.LocalOnly {
			fromInventory = append(fromInventory, operation.ID)
		}
	}

	if !reflect.DeepEqual(LocalOnlyOperations(), fromInventory) {
		t.Fatalf("local-only operations = %#v, want %#v", LocalOnlyOperations(), fromInventory)
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
