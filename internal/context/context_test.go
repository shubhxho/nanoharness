package context

import (
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "src", "limiter.go"), []byte("func EnforceLimit() {}\n// inbound webhook limiter\n"), 0644)
	os.WriteFile(filepath.Join(root, "src", "handler.go"), []byte("func webhook() { EnforceLimit() }\n"), 0644)
	os.MkdirAll(filepath.Join(root, "target"), 0755)
	os.WriteFile(filepath.Join(root, "target", "ignored.go"), []byte("EnforceLimit"), 0644)
	return root
}
func TestSearchReturnsCitedLocalMatches(t *testing.T) {
	root := fixture(t)
	r, err := Search(root, "webhook limiter")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Citations) != 1 {
		t.Fatalf("got %d citations", len(r.Citations))
	}
	c := r.Citations[0]
	if c.Path != "src/limiter.go" || c.StartLine != 1 {
		t.Fatalf("bad citation %+v", c)
	}
	if c.Score == 0 {
		t.Fatal("expected score")
	}
}
func TestSearchSkipsBuildOutput(t *testing.T) {
	root := fixture(t)
	r, err := Search(root, "EnforceLimit")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range r.Citations {
		if c.Path == "target/ignored.go" {
			t.Fatal("searched target")
		}
	}
}
func TestRenderMarksSourceUntrusted(t *testing.T) {
	root := fixture(t)
	r, _ := Search(root, "webhook")
	out := Render(r.Citations)
	if !contains(out, "untrusted reference") || !contains(out, "src/limiter.go") {
		t.Fatalf("bad context %q", out)
	}
}
func contains(s, part string) bool {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
