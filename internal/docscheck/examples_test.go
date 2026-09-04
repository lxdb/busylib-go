package docscheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestEveryServiceHasACompilingExample(t *testing.T) {
	root := repositoryRoot(t)
	accessors, _ := exportedServices(t, root)

	exampleFiles, err := filepath.Glob(filepath.Join(root, "*_test.go"))
	if err != nil {
		t.Fatalf("list root example files: %v", err)
	}
	examples := make(map[string][]*ast.FuncDecl)
	for _, path := range exampleFiles {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || !strings.HasPrefix(function.Name.Name, "Example") {
				continue
			}
			examples[function.Name.Name] = append(examples[function.Name.Name], function)
		}
	}

	for _, accessor := range slices.Sorted(maps.Keys(accessors)) {
		serviceType := accessors[accessor]
		prefix := "Example" + serviceType + "_"
		covered := false
		for name, functions := range examples {
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			for _, function := range functions {
				if exampleCallsServiceAccessor(function, accessor) {
					covered = true
					break
				}
			}
		}
		if !covered {
			t.Errorf("%s has no compiling example that calls client.%s()", serviceType, accessor)
		}
	}
}

func TestExampleCallsServiceAccessor(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "example_test.go", `package example
func ExampleSystemService_Status() {
	client.System().Status(ctx)
}
func ExampleSystemService_empty() {}
func ExampleSystemService_wrongReceiver() {
	other.System()
}
`, 0)
	if err != nil {
		t.Fatalf("parse example fixture: %v", err)
	}

	var functions []*ast.FuncDecl
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok {
			functions = append(functions, function)
		}
	}
	if !exampleCallsServiceAccessor(functions[0], "System") {
		t.Error("example that calls client.System() was not recognized")
	}
	if exampleCallsServiceAccessor(functions[1], "System") {
		t.Error("empty example was recognized as service coverage")
	}
	if exampleCallsServiceAccessor(functions[2], "System") {
		t.Error("accessor call on a receiver other than client was recognized as service coverage")
	}
}

func exampleCallsServiceAccessor(function *ast.FuncDecl, accessor string) bool {
	found := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != accessor {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if ok && receiver.Name == "client" {
			found = true
			return false
		}
		return true
	})
	return found
}
