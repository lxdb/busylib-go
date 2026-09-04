package docscheck

import (
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestPublishedPackagesHaveDocumentation(t *testing.T) {
	root := repositoryRoot(t)
	for _, relative := range publishedPackageDirectories(t, root) {
		t.Run(relative, func(t *testing.T) {
			directory := filepath.Join(root, filepath.FromSlash(relative))
			packages, err := parser.ParseDir(token.NewFileSet(), directory, func(info os.FileInfo) bool {
				return !strings.HasSuffix(info.Name(), "_test.go")
			}, parser.ParseComments)
			if err != nil {
				t.Fatalf("parse package: %v", err)
			}
			if len(packages) != 1 {
				t.Fatalf("found %d packages, want 1", len(packages))
			}
			for importName, parsed := range packages {
				if strings.TrimSpace(doc.New(parsed, relative, 0).Doc) == "" {
					t.Errorf("package %s has no package documentation", importName)
				}
			}
		})
	}
}

func publishedPackageDirectories(t *testing.T, root string) []string {
	t.Helper()
	directories := make(map[string]struct{})
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path == root {
				return nil
			}
			if isUnpublishedDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		directories[filepath.ToSlash(relative)] = struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("discover published packages: %v", err)
	}

	result := make([]string, 0, len(directories))
	for directory := range directories {
		result = append(result, directory)
	}
	sort.Strings(result)
	return result
}

func isUnpublishedDirectory(name string) bool {
	return strings.HasPrefix(name, ".") ||
		name == "experiments" ||
		name == "internal" ||
		name == "integration" ||
		name == "testdata"
}
