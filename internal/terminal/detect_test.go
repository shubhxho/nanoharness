package terminal

import (
	"testing"
)

func TestDetectGhostty(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "ghostty")
	t.Setenv("TERM_PROGRAM_VERSION", "1.2.0")
	t.Setenv("TERM", "xterm-ghostty")
	t.Setenv("COLORTERM", "truecolor")

	info := Detect()
	if !info.Ghostty {
		t.Fatal("expected ghostty")
	}
	if !info.TrueColor {
		t.Fatal("expected truecolor")
	}
	if got := info.Label(); got != "ghostty 1.2.0" {
		t.Fatalf("label = %q", got)
	}
	if got := info.Summary(); got != "ghostty 1.2.0 · truecolor" {
		t.Fatalf("summary = %q", got)
	}
}

func TestDetectFallbackTerm(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("TERM_PROGRAM_VERSION", "")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "")

	info := Detect()
	if info.Ghostty {
		t.Fatal("unexpected ghostty")
	}
	if got := info.Label(); got != "xterm-256color" {
		t.Fatalf("label = %q", got)
	}
}
