package api

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOpenAPIInventoryMatchesFixture(t *testing.T) {
	actual := readOpenAPI(t)
	expected := readInventoryFixture(t)

	if actual.OpenAPIVersion != ExpectedOpenAPIVersion {
		t.Fatalf("openapi version = %q, want %q", actual.OpenAPIVersion, ExpectedOpenAPIVersion)
	}
	if actual.OperationCount != ExpectedOperationCount {
		t.Fatalf("operation count = %d, want %d", actual.OperationCount, ExpectedOperationCount)
	}
	if actual.LocalOnlyCount != ExpectedLocalOnlyCount {
		t.Fatalf("local-only count = %d, want %d", actual.LocalOnlyCount, ExpectedLocalOnlyCount)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("inventory drift: regenerate with scripts/generate-openapi-inventory.sh")
	}
}

func TestOpenAPIInventoryCapturesContractShape(t *testing.T) {
	inventory := readOpenAPI(t)

	assertSecurityScheme(t, inventory, SecurityScheme{
		ID:   "ApiKeyAuth",
		Type: "apiKey",
		In:   "header",
		Name: "X-API-Token",
	})

	assetUpload := findOperation(t, inventory, "POST /api/assets/upload")
	assertParameter(t, assetUpload, Parameter{
		Name:     "application_name",
		In:       "query",
		Required: true,
		Schema:   "string",
	})
	assertParameter(t, assetUpload, Parameter{
		Name:     "file",
		In:       "query",
		Required: true,
		Schema:   "string",
	})
	assertRequestBodyContent(t, assetUpload, "application/octet-stream", "string(binary)")
	assertResponse(t, assetUpload, "413", "application/json", "#/components/schemas/Error")

	setBusyProfile := findOperation(t, inventory, "PUT /api/busy/profiles/{slot}")
	assertParameter(t, setBusyProfile, Parameter{
		Name:     "slot",
		In:       "path",
		Required: true,
		Schema:   "#/components/schemas/BusyProfileSlot",
	})
	assertRequestBodyContent(t, setBusyProfile, "application/json", "#/components/schemas/BusyProfile")

	wifiConnect := findOperation(t, inventory, "POST /api/wifi/connect")
	if wifiConnect.ID != "POST /api/wifi/connect" {
		t.Fatalf("wifi connect id = %q", wifiConnect.ID)
	}
	if wifiConnect.OperationID != "" {
		t.Fatalf("wifi connect operation id = %q, want empty", wifiConnect.OperationID)
	}
}

func readOpenAPI(t *testing.T) Inventory {
	t.Helper()

	inventory, err := BuildInventoryFile("testdata/busybar-f21-openapi-1.0.0-rc.yaml")
	if err != nil {
		t.Fatalf("read openapi fixture: %v", err)
	}
	return inventory
}

func readInventoryFixture(t *testing.T) Inventory {
	t.Helper()

	data, err := os.ReadFile("testdata/operations.json")
	if err != nil {
		t.Fatalf("read inventory fixture: %v", err)
	}

	var fixture Inventory
	if err := UnmarshalInventory(data, &fixture); err != nil {
		t.Fatalf("parse inventory fixture: %v", err)
	}
	return fixture
}

func UnmarshalInventory(data []byte, inventory *Inventory) error {
	return json.Unmarshal(data, inventory)
}

func findOperation(t *testing.T, inventory Inventory, id string) Operation {
	t.Helper()
	for _, operation := range inventory.Operations {
		if operation.ID == id {
			return operation
		}
	}
	t.Fatalf("operation %q not found", id)
	return Operation{}
}

func assertSecurityScheme(t *testing.T, inventory Inventory, expected SecurityScheme) {
	t.Helper()
	for _, scheme := range inventory.SecuritySchemes {
		if scheme == expected {
			return
		}
	}
	t.Fatalf("security scheme %#v not found in %#v", expected, inventory.SecuritySchemes)
}

func assertParameter(t *testing.T, operation Operation, expected Parameter) {
	t.Helper()
	for _, parameter := range operation.Parameters {
		if parameter == expected {
			return
		}
	}
	t.Fatalf("parameter %#v not found in %#v", expected, operation.Parameters)
}

func assertRequestBodyContent(t *testing.T, operation Operation, mediaType, schema string) {
	t.Helper()
	if operation.RequestBody == nil {
		t.Fatalf("%s has no request body", operation.ID)
	}
	assertContent(t, operation.RequestBody.Content, mediaType, schema)
}

func assertResponse(t *testing.T, operation Operation, status, mediaType, schema string) {
	t.Helper()
	for _, response := range operation.Responses {
		if response.Status == status {
			assertContent(t, response.Content, mediaType, schema)
			return
		}
	}
	t.Fatalf("response %s not found in %#v", status, operation.Responses)
}

func assertContent(t *testing.T, content []Content, mediaType, schema string) {
	t.Helper()
	for _, item := range content {
		if item.MediaType == mediaType && item.Schema == schema {
			return
		}
	}
	t.Fatalf("content %s %s not found in %#v", mediaType, schema, content)
}
