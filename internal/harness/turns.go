package harness

import (
	"fmt"
	"strings"
	"time"
)

const maxTurnHistory = 24

// Turn is one completed harness gather → send cycle.
type Turn struct {
	Prompt    string
	Preview   string
	Provider  string
	Cites     int
	GatherFor time.Duration
	SendFor   time.Duration
	At        time.Time
	OK        bool
}

func preview(text string, n int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if len(text) <= n {
		return text
	}
	return text[:n] + "…"
}

func (s *Session) addTurn(packet Packet, text string, sendFor time.Duration, ok bool) {
	t := Turn{
		Prompt:    packet.Prompt,
		Preview:   preview(text, 120),
		Provider:  s.Provider,
		Cites:     packet.CiteCount,
		GatherFor: s.Stats.LastGather,
		SendFor:   sendFor,
		At:        time.Now(),
		OK:        ok,
	}
	if !ok && text != "" {
		t.Preview = preview(text, 120)
	}
	s.Turns = append(s.Turns, t)
	if len(s.Turns) > maxTurnHistory {
		s.Turns = s.Turns[len(s.Turns)-maxTurnHistory:]
	}
}

// LastTurn returns the most recent turn, if any.
func (s *Session) LastTurn() (Turn, bool) {
	if len(s.Turns) == 0 {
		return Turn{}, false
	}
	return s.Turns[len(s.Turns)-1], true
}

// FormatLastTurn is a compact summary for TUI / CLI.
func FormatLastTurn(t Turn) string {
	status := "ok"
	if !t.OK {
		status = "failed"
	}
	return fmt.Sprintf("%s · %s · %d cites · gather %s · send %s · %s",
		status, t.Provider, t.Cites,
		t.GatherFor.Round(time.Millisecond),
		t.SendFor.Round(time.Millisecond),
		preview(t.Prompt, 60),
	)
}
