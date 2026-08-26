package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/shubhxho/nanoharness/internal/harness"
)

func renderChat(messages []message, busy bool, p phase, spin string, started time.Time) string {
	peach := lipgloss.Color("#fab387")
	teal := lipgloss.Color("#94e2d5")
	red := lipgloss.Color("#f38ba8")
	yellow := lipgloss.Color("#f9e2af")
	lav := lipgloss.Color("#b4befe")
	var b strings.Builder
	for _, msg := range messages {
		c := peach
		switch {
		case msg.err:
			c = red
		case msg.role == "you":
			c = teal
		case msg.role == "context":
			c = yellow
		}
		b.WriteString(lipgloss.NewStyle().Foreground(c).Bold(true).Render("▎ "+strings.ToUpper(msg.role)) + "\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4")).Render(clipText(msg.text, 4000)) + "\n\n")
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
		b.WriteString(lipgloss.NewStyle().Foreground(lav).Italic(true).Render(spin + " " + label + elapsed))
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
