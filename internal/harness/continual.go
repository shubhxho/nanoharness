package harness

import (
	"fmt"
	"strings"
)

const (
	maxMemories     = 12
	maxMemoryBytes  = 400
	defaultMaxTurns = 12
)

// Continual is durable working-state inspired by Prime Agent's Continual Harness:
// goals, short memories, and bounded autonomous execution. It is owned by
// Session and applied on Gather/Send — not a fork of Prime Agent code.
type Continual struct {
	Goal       string
	Memories   []string
	Autonomous bool
	Gates      []string
	MaxTurns   int
}

func (c Continual) preamble() string {
	var b strings.Builder
	if g := strings.TrimSpace(c.Goal); g != "" {
		fmt.Fprintf(&b, "Persistent goal (Continual Harness): %s\nComplete or clearly report blockers against this goal.\n\n", g)
	}
	if len(c.Memories) > 0 {
		b.WriteString("Session memories (evidence-backed notes; incomplete):\n")
		for i, m := range c.Memories {
			fmt.Fprintf(&b, "- %d. %s\n", i+1, m)
		}
		b.WriteString("\n")
	}
	if c.Autonomous {
		fmt.Fprintf(&b, "Autonomous mode is requested (max turns %d). Prefer finishing against gates when provided.\n\n", c.maxTurns())
	}
	return b.String()
}

func (c Continual) maxTurns() int {
	if c.MaxTurns > 0 {
		return c.MaxTurns
	}
	return defaultMaxTurns
}

func (c *Continual) remember(note string) {
	note = strings.TrimSpace(note)
	if note == "" {
		return
	}
	if len(note) > maxMemoryBytes {
		note = note[:maxMemoryBytes] + "…"
	}
	c.Memories = append(c.Memories, note)
	if len(c.Memories) > maxMemories {
		c.Memories = c.Memories[len(c.Memories)-maxMemories:]
	}
}

func (c *Continual) addGate(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	for _, g := range c.Gates {
		if g == cmd {
			return
		}
	}
	c.Gates = append(c.Gates, cmd)
}
