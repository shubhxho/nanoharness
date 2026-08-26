package harness

import (
	"fmt"
	"strings"
)

// ContinualSummary is a compact one-line continual harness snapshot.
func ContinualSummary(c Continual) string {
	parts := []string{}
	if g := strings.TrimSpace(c.Goal); g != "" {
		parts = append(parts, "goal")
	}
	if c.Autonomous {
		parts = append(parts, fmt.Sprintf("auto×%d", c.TurnLimit()))
	}
	if n := len(c.Gates); n > 0 {
		parts = append(parts, fmt.Sprintf("%d gates", n))
	}
	if n := len(c.Memories); n > 0 {
		parts = append(parts, fmt.Sprintf("%d memories", n))
	}
	if len(parts) == 0 {
		return "continual idle"
	}
	return strings.Join(parts, " · ")
}

// ContinualDetail lists goal, gates, and memories for status screens.
func ContinualDetail(c Continual) string {
	var b strings.Builder
	goal := strings.TrimSpace(c.Goal)
	if goal == "" {
		goal = "(none)"
	}
	fmt.Fprintf(&b, "goal      %s\n", goal)
	fmt.Fprintf(&b, "auto      %t (turns %d)\n", c.Autonomous, c.TurnLimit())
	if len(c.Gates) == 0 {
		b.WriteString("gates     (none)\n")
	} else {
		b.WriteString("gates\n")
		for i, g := range c.Gates {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, g)
		}
	}
	if len(c.Memories) == 0 {
		b.WriteString("memories  (none)\n")
	} else {
		b.WriteString("memories\n")
		for i, m := range c.Memories {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, m)
		}
	}
	return b.String()
}
