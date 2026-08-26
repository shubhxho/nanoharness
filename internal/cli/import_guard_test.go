package cli_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Ensure installable app packages only reach providers/context via harness.
func TestAppImportsStayOnHarnessPath(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	dirs := []string{
		filepath.Join(root, "cmd", "nanoharness"),
		filepath.Join(root, "internal", "cli"),
		filepath.Join(root, "internal", "tui"),
	}
	fset := token.NewFileSet()
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range f.Imports {
				ipath := strings.Trim(imp.Path.Value, `"`)
				if strings.HasSuffix(ipath, "/internal/providers") || strings.HasSuffix(ipath, "/internal/context") {
					t.Fatalf("%s imports %s directly; use internal/harness", path, ipath)
				}
			}
		}
	}
}
