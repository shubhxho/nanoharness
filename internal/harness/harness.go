// Package harness is the Superpower pipeline: every provider ask can gather
// local lexical citations and leave the machine only through one gated path.
package harness

import (
	"fmt"
	"os"
	"strings"

	local "github.com/shubhxho/nanoharness/internal/context"
	"github.com/shubhxho/nanoharness/internal/providers"
)

// Config drives a single Superpower-aware request.
type Config struct {
	Super    bool
	Root     string
	Provider string
	Model    string
	Write    bool
	// Preloaded citations (from /query etc). Merged with Super gather results.
	Evidence []local.Citation
	Attach   bool // attach citations even without Super
	Limit    int
}

// Packet is a prompt ready to send (or confirm) through the harness.
type Packet struct {
	Prompt    string
	Wire      string
	Citations []local.Citation
	Report    local.Report
	Confirm   bool
	CiteCount int
	Terms     []string
	Gathered  bool
}

// Gather builds the wire prompt: optional local search + citation attachment.
func Gather(cfg Config, prompt string) (Packet, error) {
	prompt = strings.TrimSpace(prompt)
	packet := Packet{Prompt: prompt, Wire: prompt}
	if prompt == "" {
		return packet, fmt.Errorf("prompt is empty")
	}

	limit := cfg.Limit
	if limit <= 0 {
		limit = local.AttachLimit
	}
	root := cfg.Root
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return packet, err
		}
		root = cwd
	}

	var fresh []local.Citation
	if cfg.Super {
		terms := local.ExtractTerms(prompt)
		packet.Terms = terms
		query := strings.Join(terms, " ")
		if query == "" {
			query = prompt
		}
		report, err := local.SearchMode(root, query, local.ModeResearch)
		if err != nil {
			return packet, err
		}
		packet.Report = report
		packet.Gathered = true
		fresh = report.Citations
	}

	citations := local.Top(local.MergeCitations(cfg.Evidence, fresh), limit)
	packet.Citations = citations
	packet.CiteCount = len(citations)

	attach := packet.CiteCount > 0 && (cfg.Super || cfg.Attach)
	if attach {
		packet.Wire = local.SuperPreamble(packet.CiteCount) + prompt + "\n\n" + local.Render(citations)
		packet.Confirm = true
	}
	if cfg.Write {
		packet.Confirm = true
	}
	return packet, nil
}

// Send delivers an already-gathered packet to the selected provider.
func Send(cfg Config, packet Packet) (string, error) {
	wire := packet.Wire
	if wire == "" {
		wire = packet.Prompt
	}
	return providers.Ask(cfg.Provider, wire, cfg.Model, cfg.Write)
}

// Run gathers then sends in one shot (CLI path).
func Run(cfg Config, prompt string) (string, Packet, error) {
	packet, err := Gather(cfg, prompt)
	if err != nil {
		return "", packet, err
	}
	text, err := Send(cfg, packet)
	return text, packet, err
}

// StatusLine is a compact inspector string.
func StatusLine(cfg Config, packet Packet) string {
	mode := "off"
	if cfg.Super {
		mode = "on"
	}
	return fmt.Sprintf("super %s · %d cites · write %t", mode, packet.CiteCount, cfg.Write)
}
