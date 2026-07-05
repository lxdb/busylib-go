package api

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ExpectedOpenAPIVersion = "24.3.0"
	ExpectedOperationCount = 68
	ExpectedLocalOnlyCount = 6
)

type Inventory struct {
	OpenAPIVersion  string           `json:"openapiVersion"`
	OperationCount  int              `json:"operationCount"`
	LocalOnlyCount  int              `json:"localOnlyCount"`
	SecuritySchemes []SecurityScheme `json:"securitySchemes"`
	Operations      []Operation      `json:"operations"`
}

type SecurityScheme struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	In   string `json:"in,omitempty"`
	Name string `json:"name,omitempty"`
}

type Operation struct {
	ID          string       `json:"id"`
	OperationID string       `json:"operationId,omitempty"`
	Method      string       `json:"method"`
	Path        string       `json:"path"`
	Tags        []string     `json:"tags"`
	LocalOnly   bool         `json:"localOnly"`
	Parameters  []Parameter  `json:"parameters"`
	RequestBody *RequestBody `json:"requestBody,omitempty"`
	Responses   []Response   `json:"responses"`
}

type Parameter struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required"`
	Schema   string `json:"schema,omitempty"`
}

type RequestBody struct {
	Required bool      `json:"required"`
	Content  []Content `json:"content"`
}

type Response struct {
	Status      string    `json:"status"`
	Description string    `json:"description,omitempty"`
	Content     []Content `json:"content"`
}

type Content struct {
	MediaType string `json:"mediaType"`
	Schema    string `json:"schema,omitempty"`
}

type openAPIDoc struct {
	Info struct {
		Version string `yaml:"version"`
	} `yaml:"info"`
	Components struct {
		SecuritySchemes map[string]openAPISecurityScheme `yaml:"securitySchemes"`
	} `yaml:"components"`
	Paths map[string]map[string]openAPIOperation `yaml:"paths"`
}

type openAPISecurityScheme struct {
	Type string `yaml:"type"`
	In   string `yaml:"in"`
	Name string `yaml:"name"`
}

type openAPIOperation struct {
	Tags        []string                   `yaml:"tags"`
	OperationID string                     `yaml:"operationId"`
	LocalOnly   bool                       `yaml:"x-local-only"`
	Parameters  []openAPIParameter         `yaml:"parameters"`
	RequestBody *openAPIRequestBody        `yaml:"requestBody"`
	Responses   map[string]openAPIResponse `yaml:"responses"`
}

type openAPIParameter struct {
	Name     string         `yaml:"name"`
	In       string         `yaml:"in"`
	Required bool           `yaml:"required"`
	Schema   *openAPISchema `yaml:"schema"`
}

type openAPIRequestBody struct {
	Required bool                    `yaml:"required"`
	Content  map[string]openAPIMedia `yaml:"content"`
}

type openAPIResponse struct {
	Description string                  `yaml:"description"`
	Content     map[string]openAPIMedia `yaml:"content"`
}

type openAPIMedia struct {
	Schema *openAPISchema `yaml:"schema"`
}

type openAPISchema struct {
	Ref    string         `yaml:"$ref"`
	Type   string         `yaml:"type"`
	Format string         `yaml:"format"`
	Items  *openAPISchema `yaml:"items"`
}

func BuildInventoryFile(path string) (Inventory, error) {
	file, err := os.Open(path)
	if err != nil {
		return Inventory{}, err
	}
	defer file.Close()

	return BuildInventory(file)
}

func BuildInventory(reader io.Reader) (Inventory, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return Inventory{}, err
	}

	var doc openAPIDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Inventory{}, err
	}
	if doc.Paths == nil {
		return Inventory{}, fmt.Errorf("openapi paths are missing")
	}

	operations := make([]Operation, 0, ExpectedOperationCount)
	for path, methods := range doc.Paths {
		for method, op := range methods {
			if !isHTTPMethod(method) {
				continue
			}
			method = strings.ToUpper(method)
			operations = append(operations, Operation{
				ID:          method + " " + path,
				OperationID: op.OperationID,
				Method:      method,
				Path:        path,
				Tags:        append([]string(nil), op.Tags...),
				LocalOnly:   op.LocalOnly,
				Parameters:  buildParameters(op.Parameters),
				RequestBody: buildRequestBody(op.RequestBody),
				Responses:   buildResponses(op.Responses),
			})
		}
	}

	sort.Slice(operations, func(i, j int) bool {
		if operations[i].Path != operations[j].Path {
			return operations[i].Path < operations[j].Path
		}
		return methodRank(operations[i].Method) < methodRank(operations[j].Method)
	})

	localOnlyCount := 0
	for _, op := range operations {
		if op.LocalOnly {
			localOnlyCount++
		}
	}

	return Inventory{
		OpenAPIVersion: doc.Info.Version,
		OperationCount: len(operations),
		LocalOnlyCount: localOnlyCount,
		SecuritySchemes: buildSecuritySchemes(
			doc.Components.SecuritySchemes,
		),
		Operations: operations,
	}, nil
}

func WriteInventory(writer io.Writer, inventory Inventory) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(inventory)
}

func buildSecuritySchemes(source map[string]openAPISecurityScheme) []SecurityScheme {
	schemes := make([]SecurityScheme, 0, len(source))
	for id, scheme := range source {
		schemes = append(schemes, SecurityScheme{
			ID:   id,
			Type: scheme.Type,
			In:   scheme.In,
			Name: scheme.Name,
		})
	}
	sort.Slice(schemes, func(i, j int) bool {
		return schemes[i].ID < schemes[j].ID
	})
	return schemes
}

func buildParameters(source []openAPIParameter) []Parameter {
	parameters := make([]Parameter, 0, len(source))
	for _, parameter := range source {
		parameters = append(parameters, Parameter{
			Name:     parameter.Name,
			In:       parameter.In,
			Required: parameter.Required,
			Schema:   schemaIdentity(parameter.Schema),
		})
	}
	return parameters
}

func buildRequestBody(source *openAPIRequestBody) *RequestBody {
	if source == nil {
		return nil
	}
	return &RequestBody{
		Required: source.Required,
		Content:  buildContent(source.Content),
	}
}

func buildResponses(source map[string]openAPIResponse) []Response {
	responses := make([]Response, 0, len(source))
	for status, response := range source {
		responses = append(responses, Response{
			Status:      status,
			Description: response.Description,
			Content:     buildContent(response.Content),
		})
	}
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].Status < responses[j].Status
	})
	return responses
}

func buildContent(source map[string]openAPIMedia) []Content {
	content := make([]Content, 0, len(source))
	for mediaType, media := range source {
		content = append(content, Content{
			MediaType: mediaType,
			Schema:    schemaIdentity(media.Schema),
		})
	}
	sort.Slice(content, func(i, j int) bool {
		return content[i].MediaType < content[j].MediaType
	})
	return content
}

func schemaIdentity(schema *openAPISchema) string {
	if schema == nil {
		return ""
	}
	if schema.Ref != "" {
		return schema.Ref
	}
	identity := schema.Type
	if schema.Format != "" {
		identity += "(" + schema.Format + ")"
	}
	if schema.Items != nil {
		identity += "[" + schemaIdentity(schema.Items) + "]"
	}
	return identity
}

func isHTTPMethod(method string) bool {
	switch strings.ToLower(method) {
	case "delete", "get", "patch", "post", "put":
		return true
	default:
		return false
	}
}

func methodRank(method string) int {
	switch method {
	case "DELETE":
		return 0
	case "GET":
		return 1
	case "PATCH":
		return 2
	case "POST":
		return 3
	case "PUT":
		return 4
	default:
		return 100
	}
}
