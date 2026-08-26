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

// Run gathers then sends in one shot. Always goes through Gather → Send.
func Run(cfg Config, prompt string) (Result, error) {
	packet, err := Gather(cfg, prompt)
	if err != nil {
		return Result{Packet: packet}, err
	}
	text, err := Send(cfg, packet)
	return Result{Text: text, Packet: packet}, err
}

// RunTimed is Run with gather/send durations for CLI progress lines.
func RunTimed(cfg Config, prompt string) (Result, time.Duration, time.Duration, error) {
	start := time.Now()
	packet, err := Gather(cfg, prompt)
	gatherFor := time.Since(start)
	if err != nil {
		return Result{Packet: packet}, gatherFor, 0, err
	}
	start = time.Now()
	text, err := Send(cfg, packet)
	return Result{Text: text, Packet: packet}, gatherFor, time.Since(start), err
}
