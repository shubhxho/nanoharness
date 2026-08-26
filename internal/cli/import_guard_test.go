package cli_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Ensure the installable binary only reaches providers/context via harness packages.
func TestCmdImportsStayOnHarnessPath(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	paths := []string{
		filepath.Join(root, "cmd", "nanoharness", "main.go"),
		filepath.Join(root, "internal", "cli", "cli.go"),
		filepath.Join(root, "internal", "tui", "tui.go"),
	}
	fset := token.NewFileSet()
	for _, path := range paths {
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.HasSuffix(path, "/internal/providers") || strings.HasSuffix(path, "/internal/context") {
				t.Fatalf("%s imports %s directly; use internal/harness", filepath.Base(path), path)
			}
		}
	}
}
