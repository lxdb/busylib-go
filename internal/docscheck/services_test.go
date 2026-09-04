package docscheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	internalapi "github.com/lxdb/busylib-go/internal/api"
)

const moduleURL = "https://pkg.go.dev/github.com/lxdb/busylib-go"

var (
	serviceRowPattern     = regexp.MustCompile("(?m)^\\| \\[`(client\\.([A-Z][A-Za-z0-9]*)\\(\\)\\.([A-Z][A-Za-z0-9]*))`\\]\\((https://pkg\\.go\\.dev/[^)]+)\\) \\| [^|\\n]* \\| ([^|\\n]*) \\|")
	restrictionRowPattern = regexp.MustCompile("(?m)^\\| `(client\\.[A-Z][A-Za-z0-9]*\\(\\)\\.[A-Z][A-Za-z0-9]*)` \\| `((?:DELETE|GET|PATCH|POST|PUT) /api/[^`]+)` \\|$")
)

func TestServiceReferenceCoversPublicServices(t *testing.T) {
	root := repositoryRoot(t)
	accessors, methods := exportedServices(t, root)
	content := readFile(t, filepath.Join(root, "docs/reference/services.md"))

	documented := make(map[string]int)
	for _, row := range parseServiceReferenceRows(content) {
		serviceType, ok := accessors[row.Accessor]
		if !ok {
			t.Errorf("service reference contains unknown accessor %q", row.Accessor)
			continue
		}
		wantTarget := moduleURL + "#" + serviceType + "." + row.Method
		if row.Target != wantTarget {
			t.Errorf("%s links to %q, want %q", row.Call, row.Target, wantTarget)
		}
		if row.RemoteText != "Yes" && row.RemoteText != "No; local only" {
			t.Errorf("%s has invalid Remote MQTT value %q", row.Call, row.RemoteText)
		}
		documented[row.Call]++
	}

	want := make(map[string]struct{})
	for accessor, serviceType := range accessors {
		for _, method := range methods[serviceType] {
			want["client."+accessor+"()."+method] = struct{}{}
		}
	}
	for _, call := range slices.Sorted(maps.Keys(want)) {
		switch documented[call] {
		case 0:
			t.Errorf("service reference is missing %s", call)
		case 1:
		default:
			t.Errorf("service reference lists %s %d times", call, documented[call])
		}
	}
	for _, call := range slices.Sorted(maps.Keys(documented)) {
		if _, ok := want[call]; !ok {
			t.Errorf("service reference contains unknown method %s", call)
		}
	}
}

func TestServiceReferenceListsRemoteBlockedOperations(t *testing.T) {
	content := readFile(t, filepath.Join(repositoryRoot(t), "docs/reference/services.md"))
	restrictionMethods := make(map[string]int)
	documentedOperations := make(map[string]int)
	for _, match := range restrictionRowPattern.FindAllStringSubmatch(content, -1) {
		restrictionMethods[match[1]]++
		documentedOperations[match[2]]++
	}

	localOnlyMethods := make(map[string]int)
	for _, row := range parseServiceReferenceRows(content) {
		if !row.Remote {
			localOnlyMethods[row.Call]++
		}
	}
	for _, method := range slices.Sorted(maps.Keys(localOnlyMethods)) {
		if restrictionMethods[method] != 1 {
			t.Errorf("local-only method %q appears %d times in the restriction table, want once", method, restrictionMethods[method])
		}
	}
	for _, method := range slices.Sorted(maps.Keys(restrictionMethods)) {
		if localOnlyMethods[method] != 1 {
			t.Errorf("restriction method %q appears %d times as local-only in the service rows, want once", method, localOnlyMethods[method])
		}
	}

	want := make(map[string]struct{})
	for _, operation := range internalapi.RemoteBlockedOperations() {
		want[operation] = struct{}{}
	}
	for _, operation := range slices.Sorted(maps.Keys(want)) {
		if documentedOperations[operation] != 1 {
			t.Errorf("service reference lists remote-blocked operation %q %d times, want once", operation, documentedOperations[operation])
		}
	}
	for _, operation := range slices.Sorted(maps.Keys(documentedOperations)) {
		if _, ok := want[operation]; !ok {
			t.Errorf("service reference lists %q as remote-blocked, but the firmware metadata does not", operation)
		}
	}
}

func TestParseServiceReferenceRowsIncludesRemoteAvailability(t *testing.T) {
	rows := parseServiceReferenceRows("\n" +
		"| Method | Effect | Remote MQTT | Guide |\n" +
		"| --- | --- | --- | --- |\n" +
		"| [`client.WiFi().Connect`](https://pkg.go.dev/github.com/lxdb/busylib-go#WiFiService.Connect) | Connect. | No; local only | Guide |\n" +
		"| [`client.WiFi().Status`](https://pkg.go.dev/github.com/lxdb/busylib-go#WiFiService.Status) | Read. | Yes | Guide |\n")
	if len(rows) != 2 {
		t.Fatalf("parsed %d service rows, want 2", len(rows))
	}
	if rows[0].Remote {
		t.Error("local-only row was parsed as remote-capable")
	}
	if !rows[1].Remote {
		t.Error("remote-capable row was parsed as local-only")
	}
}

func TestExportedServicesScansAllPackageFiles(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "client.go", "package example\ntype Client struct{}\ntype SystemService struct{}\nfunc (Client) System() SystemService { return SystemService{} }\n")
	writeFixture(t, root, "system.go", "package example\nfunc (SystemService) Status() {}\n")
	writeFixture(t, root, "system_test.go", "package example\nfunc (SystemService) TestOnly() {}\n")

	accessors, methods := exportedServices(t, root)
	if accessors["System"] != "SystemService" {
		t.Fatalf("System accessor resolved to %q", accessors["System"])
	}
	if !slices.Equal(methods["SystemService"], []string{"Status"}) {
		t.Fatalf("SystemService methods = %v, want [Status]", methods["SystemService"])
	}
}

type serviceReferenceRow struct {
	Call       string
	Accessor   string
	Method     string
	Target     string
	Remote     bool
	RemoteText string
}

func parseServiceReferenceRows(content string) []serviceReferenceRow {
	matches := serviceRowPattern.FindAllStringSubmatch(content, -1)
	rows := make([]serviceReferenceRow, 0, len(matches))
	for _, match := range matches {
		remoteText := strings.TrimSpace(match[5])
		rows = append(rows, serviceReferenceRow{
			Call: match[1], Accessor: match[2], Method: match[3], Target: match[4],
			Remote: remoteText == "Yes", RemoteText: remoteText,
		})
	}
	return rows
}

func exportedServices(t *testing.T, root string) (map[string]string, map[string][]string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, "*.go"))
	if err != nil {
		t.Fatalf("list root Go files: %v", err)
	}
	accessors := make(map[string]string)
	methods := make(map[string][]string)
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv == nil || !function.Name.IsExported() {
				continue
			}
			receiver := receiverName(function.Recv.List[0].Type)
			if receiver == "Client" && function.Type.Params.NumFields() == 0 && function.Type.Results.NumFields() == 1 {
				if result, ok := function.Type.Results.List[0].Type.(*ast.Ident); ok && strings.HasSuffix(result.Name, "Service") {
					accessors[function.Name.Name] = result.Name
				}
				continue
			}
			if strings.HasSuffix(receiver, "Service") {
				methods[receiver] = append(methods[receiver], function.Name.Name)
			}
		}
	}
	for serviceType := range methods {
		slices.Sort(methods[serviceType])
	}
	return accessors, methods
}

func receiverName(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name
	case *ast.StarExpr:
		return receiverName(expression.X)
	default:
		return ""
	}
}
