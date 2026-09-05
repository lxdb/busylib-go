package docscheck

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestObsoleteDocumentationPathsAreRemoved(t *testing.T) {
	root := repositoryRoot(t)
	for _, path := range []string{
		"docs/transports.md",
		"docs/media.md",
		"docs/compatibility.md",
		"docs/development.md",
		"docs/releasing.md",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); err == nil {
			t.Errorf("obsolete documentation path still exists: %s", path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("inspect obsolete documentation path %s: %v", path, err)
		}
	}
}

func documentationFiles(t *testing.T, root string) []string {
	t.Helper()
	files := make(map[string]struct{})
	add := func(path string) {
		if filepath.Ext(path) == ".md" {
			files[path] = struct{}{}
		}
	}

	rootFiles, err := filepath.Glob(filepath.Join(root, "*.md"))
	if err != nil {
		t.Fatalf("list root Markdown files: %v", err)
	}
	for _, path := range rootFiles {
		add(path)
	}
	for _, relativeRoot := range []string{"docs", "integration/device", "pahotransport", "ble"} {
		walkRoot := filepath.Join(root, relativeRoot)
		err := filepath.WalkDir(walkRoot, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && path == filepath.Join(root, "docs/okf") {
				return filepath.SkipDir
			}
			if !entry.IsDir() {
				add(path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", walkRoot, err)
		}
	}
	return slices.Sorted(maps.Keys(files))
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func writeFixture(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
