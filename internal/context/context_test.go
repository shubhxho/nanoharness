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

func TestExtractTermsDropsStopwords(t *testing.T) {
	terms := ExtractTerms("please find where the webhook limiter is checked")
	if len(terms) < 2 || terms[0] == "please" || terms[0] == "find" {
		t.Fatalf("bad terms %#v", terms)
	}
	joined := ""
	for _, term := range terms {
		joined += " " + term
	}
	if !contains(joined, "webhook") || !contains(joined, "limiter") {
		t.Fatalf("missing keywords %#v", terms)
	}
}

func TestResearchModeSoftMatches(t *testing.T) {
	root := fixture(t)
	r, err := SearchMode(root, "webhook missingtokenxyz", ModeResearch)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Citations) == 0 {
		t.Fatal("research should soft-match webhook")
	}
}

func TestImpactPrefersSymbolHits(t *testing.T) {
	root := fixture(t)
	r, err := SearchMode(root, "EnforceLimit", ModeImpact)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Citations) == 0 {
		t.Fatal("expected impact hits")
	}
}

func TestMergeCitationsKeepsBestPerPath(t *testing.T) {
	a := []Citation{{Path: "a.go", Score: 3, StartLine: 1, EndLine: 2}}
	b := []Citation{{Path: "a.go", Score: 9, StartLine: 4, EndLine: 8}, {Path: "b.go", Score: 2}}
	out := MergeCitations(a, b)
	if len(out) != 2 || out[0].Path != "a.go" || out[0].Score != 9 {
		t.Fatalf("bad merge %#v", out)
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
