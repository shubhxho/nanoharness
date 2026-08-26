package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	local "github.com/shubhxho/nanoharness/internal/context"
)

func TestGatherSuperAttachesLocalEvidence(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0755)
	os.WriteFile(filepath.Join(root, "src", "auth.go"), []byte("func requireUser() {}\n// auth check gate\n"), 0644)

	packet, err := Gather(Config{Super: true, Root: root, Provider: "openai"}, "where is requireUser auth checked")
	if err != nil {
		t.Fatal(err)
	}
	if packet.CiteCount == 0 {
		t.Fatal("expected citations")
	}
	if !strings.Contains(packet.Wire, "requireUser") {
		t.Fatalf("wire missing citation body: %q", packet.Wire)
	}
	if !strings.Contains(packet.Wire, "Superpower") {
		t.Fatal("expected Superpower preamble")
	}
	if !packet.Confirm {
		t.Fatal("citation attach must confirm")
	}
}

func TestGatherWithoutSuperKeepsBarePrompt(t *testing.T) {
	packet, err := Gather(Config{Super: false, Provider: "openai"}, "say hi")
	if err != nil {
		t.Fatal(err)
	}
	if packet.Wire != "say hi" || packet.CiteCount != 0 || packet.Confirm {
		t.Fatalf("unexpected packet %+v", packet)
	}
}

func TestGatherAttachUsesPreloaded(t *testing.T) {
	cites := []local.Citation{{Path: "a.go", StartLine: 1, EndLine: 2, Snippet: "1  hello", Score: 9}}
	packet, err := Gather(Config{Attach: true, Evidence: cites, Provider: "openai"}, "explain this")
	if err != nil {
		t.Fatal(err)
	}
	if packet.CiteCount != 1 || !strings.Contains(packet.Wire, "a.go") {
		t.Fatalf("bad packet %+v", packet)
	}
}

func TestGatherSuperMergesPreloaded(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0755)
	os.WriteFile(filepath.Join(root, "src", "auth.go"), []byte("func requireUser() {}\n"), 0644)
	pre := []local.Citation{{Path: "notes.md", StartLine: 1, EndLine: 1, Snippet: "1  note", Score: 1}}
	packet, err := Gather(Config{Super: true, Root: root, Evidence: pre, Provider: "openai"}, "requireUser")
	if err != nil {
		t.Fatal(err)
	}
	if packet.CiteCount < 2 {
		t.Fatalf("expected merged cites, got %d", packet.CiteCount)
	}
}
