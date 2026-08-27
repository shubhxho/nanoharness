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

func TestSearchGoesThroughHarness(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0755)
	os.WriteFile(filepath.Join(root, "src", "x.go"), []byte("webhook limiter\n"), 0644)
	r, err := Search(root, "webhook", ModeQuery)
	if err != nil || len(r.Citations) == 0 {
		t.Fatalf("search via harness failed: %v %#v", err, r)
	}
	mode, err := ParseMode("research")
	if err != nil || mode != ModeResearch {
		t.Fatalf("parse mode: %v %q", err, mode)
	}
}

func TestGatherRejectsUnknownProvider(t *testing.T) {
	_, err := Gather(Config{Provider: "nope"}, "hi")
	if err == nil {
		t.Fatal("expected provider error")
	}
}

func TestGatherSymbolPrefersImpact(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0755)
	os.WriteFile(filepath.Join(root, "src", "auth.go"), []byte("func RequireUser() {}\nRequireUser()\n"), 0644)
	packet, err := Gather(Config{Super: true, Root: root, Provider: "openai"}, "RequireUser")
	if err != nil {
		t.Fatal(err)
	}
	if packet.CiteCount == 0 {
		t.Fatal("expected symbol cites")
	}
	if packet.Mode != ModeImpact && packet.Mode != ModeResearch {
		t.Fatalf("unexpected mode %q", packet.Mode)
	}
}

func TestSessionContinualPreamble(t *testing.T) {
	session := NewSession("openai").WithGoal("ship auth").RememberNote("use sessions")
	packet, err := session.Gather("say hi")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(packet.Wire, "Persistent goal") || !strings.Contains(packet.Wire, "use sessions") {
		t.Fatalf("continual preamble missing: %q", packet.Wire)
	}
}

func TestSessionPipelineTracksStats(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0755)
	os.WriteFile(filepath.Join(root, "src", "a.go"), []byte("func Hello() {}\n"), 0644)
	session := NewSession("openai").WithRoot(root).WithSuper(true)
	packet, gatherFor, err := session.GatherTimed("Hello")
	if err != nil {
		t.Fatal(err)
	}
	if session.Stats.Gathers != 1 || gatherFor <= 0 {
		t.Fatalf("gather stats: %+v gatherFor=%s", session.Stats, gatherFor)
	}
	if len(session.Evidence) == 0 {
		t.Fatal("GatherPrepared should remember citations")
	}
	if !session.NeedsConfirm(packet) {
		t.Fatal("expected confirm for super gather")
	}
	line := session.PipelineLine()
	if !strings.Contains(line, "gathers 1") {
		t.Fatalf("pipeline line: %q", line)
	}
	status := session.Status("test")
	if !strings.Contains(status, "pipeline") || !strings.Contains(status, "gathers 1") {
		t.Fatalf("bad status %q", status)
	}
	session.Reset()
	if session.Stats.Gathers != 0 || len(session.Evidence) != 0 {
		t.Fatalf("reset failed: %+v evidence=%d", session.Stats, len(session.Evidence))
	}
}

func TestSessionSearchRemember(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "x.go"), []byte("webhook limiter\n"), 0644)
	session := NewSession("openai").WithRoot(root).WithSuper(false)
	r, err := session.SearchRemember("webhook", ModeQuery)
	if err != nil || len(r.Citations) == 0 {
		t.Fatalf("search remember: %v %#v", err, r)
	}
	if len(session.Evidence) == 0 {
		t.Fatal("expected evidence on session")
	}
}
