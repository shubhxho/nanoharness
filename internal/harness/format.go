package harness

import (
	"fmt"
	"strings"
)

// FormatReport prints a local evidence packet for CLI context commands.
func FormatReport(label, query string, mode Mode, r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s v1 — exact token/path matching only; no embeddings or dependency graph.\nquery: %s\nmode: %s\n\n", label, query, mode)
	for i, c := range r.Citations {
		fmt.Fprintf(&b, "%02d %s:%d-%d score %d\n%s\n\n", i+1, c.Path, c.StartLine, c.EndLine, c.Score, c.Snippet)
	}
	return b.String()
}

// ModeLabel is the human title for a retrieval mode.
func ModeLabel(mode Mode) string {
	switch mode {
	case ModeResearch:
		return "LOCAL LEXICAL EVIDENCE PACKET"
	case ModeImpact:
		return "POSSIBLE LEXICAL IMPACT"
	default:
		return "LOCAL LEXICAL CONTEXT"
	}
}

// Describe returns a short human status for TUI / CLI progress lines.
func Describe(packet Packet) string {
	parts := []string{fmt.Sprintf("%d cites", packet.CiteCount)}
	if packet.Gathered {
		parts = append(parts, "gathered")
	}
	if packet.Mode != "" {
		parts = append(parts, "mode "+string(packet.Mode))
	}
	if len(packet.Terms) > 0 {
		parts = append(parts, "terms "+strings.Join(packet.Terms, ","))
	}
	if packet.Confirm {
		parts = append(parts, "needs confirm")
	}
	return strings.Join(parts, " · ")
}

// ConfirmSummary is the compact gate text shown before a Superpower send.
func ConfirmSummary(cfg Config, packet Packet) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Ready to send through harness\n")
	fmt.Fprintf(&b, "provider %s · model %s · write %t · citations %d\n",
		cfg.Provider, displayOrDefault(cfg.Model), cfg.Write, packet.CiteCount)
	if packet.Mode != "" {
		fmt.Fprintf(&b, "mode: %s\n", packet.Mode)
	}
	if g := strings.TrimSpace(cfg.Continual.Goal); g != "" {
		fmt.Fprintf(&b, "goal: %s\n", g)
	}
	if cfg.Continual.Autonomous {
		fmt.Fprintf(&b, "autonomous: turns %d · gates %d\n", cfg.Continual.maxTurns(), len(cfg.Continual.Gates))
	}
	if len(packet.Terms) > 0 {
		fmt.Fprintf(&b, "terms: %s\n", strings.Join(packet.Terms, " "))
	}
	if packet.CiteCount > 0 {
		b.WriteString("evidence:\n")
		for i, c := range packet.Citations {
			if i >= 6 {
				fmt.Fprintf(&b, "  … +%d more\n", packet.CiteCount-6)
				break
			}
			fmt.Fprintf(&b, "  %02d  %s:%d-%d\n", i+1, c.Path, c.StartLine, c.EndLine)
		}
	}
	b.WriteString("y / Enter approve · n / Esc cancel")
	return b.String()
}

func displayOrDefault(model string) string {
	if model == "" {
		return "vendor default"
	}
	return model
}

// StatusLine is a compact inspector string.
func StatusLine(cfg Config, packet Packet) string {
	mode := "off"
	if cfg.Super {
		mode = "on"
	}
	return fmt.Sprintf("super %s · %d cites · write %t", mode, packet.CiteCount, cfg.Write)
}
