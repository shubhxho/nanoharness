package harness

import (
	"strings"
	"testing"
)

func TestConfirmReasons(t *testing.T) {
	cfg := DefaultConfig("openai")
	cfg.Write = true
	cfg.Continual.Autonomous = true
	packet := Packet{CiteCount: 3, Confirm: true}
	reasons := ConfirmReasons(cfg, packet)
	if len(reasons) < 3 {
		t.Fatalf("expected multiple reasons, got %v", reasons)
	}
	summary := ConfirmSummary(cfg, packet)
	if !strings.Contains(summary, "why confirm:") {
		t.Fatalf("summary missing reasons: %q", summary)
	}
}

func TestValidateSendRejectsMissingAuth(t *testing.T) {
	session := NewSession("openai")
	err := session.ValidateSend()
	if err == nil {
		t.Fatal("expected missing key error")
	}
	if !strings.Contains(err.Error(), "login openai") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTurnHistoryOnSendTimed(t *testing.T) {
	session := NewSession("openai")
	packet := Packet{Prompt: "hi", Wire: "hi"}
	// ValidateSend will fail without API key — still records failed turn
	_, _, err := session.SendTimed(packet)
	if err == nil {
		t.Fatal("expected auth error")
	}
	tn, ok := session.LastTurn()
	if !ok {
		t.Fatal("expected turn recorded")
	}
	if tn.Prompt != "hi" || tn.OK {
		t.Fatalf("unexpected turn: %+v", tn)
	}
	if !strings.Contains(FormatLastTurn(tn), "failed") {
		t.Fatalf("format: %s", FormatLastTurn(tn))
	}
}
