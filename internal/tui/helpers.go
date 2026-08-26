package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/shubhxho/nanoharness/internal/harness"
)

func renderChat(messages []message, busy bool, p phase, spin string, started time.Time) string {
	var b strings.Builder
	for _, msg := range messages {
		border := colorBorder
		switch msg.role {
		case "you":
			border = colorTeal
		case "error":
			border = colorRed
		case "context":
			border = colorYellow
		case "nano":
			border = colorLav
		default:
			border = colorPeach
		}
		head := roleStyle(msg.role).Render("▎ " + strings.ToUpper(msg.role))
		body := messageBodyStyle().Render(clipText(msg.text, 4000))
		b.WriteString(boxStyle(border).Render(head + "\n" + body))
	}
	if busy {
		label := "sending through harness…"
		if p == phaseGather {
			label = "gathering local evidence…"
		}
		elapsed := ""
		if !started.IsZero() {
			elapsed = " · " + time.Since(started).Round(time.Millisecond).String()
		}
		b.WriteString(lipgloss.NewStyle().Foreground(colorLav).Italic(true).Render(spin + " " + label + elapsed))
	}
	return b.String()
}

func pipeline(p phase) string {
	steps := []string{"gather", "confirm", "send"}
	var out []string
	for _, step := range steps {
		if string(p) == step {
			out = append(out, "["+step+"]")
		} else {
			out = append(out, step)
		}
	}
	return strings.Join(out, " → ")
}

func styledPipeline(p phase, active, confirm, send lipgloss.Color) string {
	steps := []struct {
		name  string
		phase phase
		color lipgloss.Color
	}{
		{"gather", phaseGather, active},
		{"confirm", phaseConfirm, confirm},
		{"send", phaseSend, send},
	}
	var out []string
	for _, step := range steps {
		label := step.name
		style := lipgloss.NewStyle().Foreground(colorMuted)
		if p == step.phase {
			label = "[" + step.name + "]"
			style = lipgloss.NewStyle().Foreground(step.color).Bold(true)
		}
		out = append(out, style.Render(label))
	}
	return strings.Join(out, " → ")
}

func renderConfirm(cfg harness.Config, packet harness.Packet) string {
	summary := harness.ConfirmSummary(cfg, packet)
	lines := strings.Split(summary, "\n")
	var b strings.Builder
	for i, line := range lines {
		style := messageBodyStyle()
		if i == 0 {
			style = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
		}
		if strings.HasPrefix(line, "y /") {
			style = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)
		}
		b.WriteString(style.Render(line))
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func providerIndex(id string) int {
	for i, p := range harness.Profiles {
		if p.ID == id {
			return i
		}
	}
	return 0
}

func displayModel(model string) string {
	if model == "" {
		return "vendor default"
	}
	return model
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clipText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n… truncated …"
}

func summary(c []harness.Citation) string {
	if len(c) == 0 {
		return "No local matches."
	}
	out := make([]string, len(c))
	for i, x := range c {
		out[i] = fmt.Sprintf("%02d  %s:%d-%d", i+1, x.Path, x.StartLine, x.EndLine)
	}
	return strings.Join(out, "\n")
}
