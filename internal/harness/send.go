package harness

import (
	"fmt"
	"strings"
	"time"

	"github.com/shubhxho/nanoharness/internal/providers"
)

// Send delivers an already-gathered packet to the selected provider.
// This is the only supported way to call a model from the app layer.
func Send(cfg Config, packet Packet) (string, error) {
	cfg, err := cfg.Normalize()
	if err != nil {
		return "", err
	}
	wire := packet.Wire
	if wire == "" {
		wire = packet.Prompt
	}
	if strings.TrimSpace(wire) == "" {
		return "", fmt.Errorf("prompt is empty")
	}
	return providers.Ask(cfg.Provider, wire, cfg.Model, providers.AskOptions{
		Write:      cfg.Write,
		Root:       cfg.Root,
		Goal:       cfg.Continual.Goal,
		Autonomous: cfg.Continual.Autonomous,
		Gates:      append([]string(nil), cfg.Continual.Gates...),
		MaxTurns:   cfg.Continual.maxTurns(),
	})
}

// Run gathers then sends in one shot through a Session.
func Run(cfg Config, prompt string) (Result, error) {
	s := &Session{Config: cfg}
	return s.Pipeline(prompt)
}

// RunTimed is Run with gather/send durations for CLI progress lines.
func RunTimed(cfg Config, prompt string) (Result, time.Duration, time.Duration, error) {
	s := &Session{Config: cfg}
	return s.PipelineTimed(prompt)
}
