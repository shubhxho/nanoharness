// Package harness is the single Superpower choke point for nanoharness.
// TUI and CLI must gather, search, login, and send only through this package.
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
	Evidence []local.Citation
	Attach   bool
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

// Re-exported types so cmd need not import context or providers for day-to-day work.
type (
	Citation = local.Citation
	Report   = local.Report
	Mode     = local.Mode
	Profile  = providers.Profile
)

const (
	ModeQuery    = local.ModeQuery
	ModeResearch = local.ModeResearch
	ModeImpact   = local.ModeImpact
	AttachLimit  = local.AttachLimit
)

// Profiles is the provider catalog.
var Profiles = providers.Profiles

func Find(id string) (Profile, bool) { return providers.Find(id) }

func AuthStatus(provider string) string { return providers.AuthStatus(provider) }

func Login(kind string, apiKey bool) error { return providers.Login(kind, apiKey) }

func Top(citations []Citation, n int) []Citation { return local.Top(citations, n) }

func ParseMode(name string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "query":
		return ModeQuery, nil
	case "research":
		return ModeResearch, nil
	case "impact":
		return ModeImpact, nil
	default:
		return "", fmt.Errorf("mode must be query, research, or impact")
	}
}

func resolveRoot(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	return os.Getwd()
}

// Search runs local lexical retrieval through the harness.
func Search(root, query string, mode Mode) (Report, error) {
	root, err := resolveRoot(root)
	if err != nil {
		return Report{}, err
	}
	if mode == "" {
		mode = ModeQuery
	}
	return local.SearchMode(root, query, mode)
}

// Index scans the tree once and reports bounds (no meaningful query).
func Index(root string) (Report, error) {
	root, err := resolveRoot(root)
	if err != nil {
		return Report{}, err
	}
	return local.Search(root, "index")
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
	root, err := resolveRoot(cfg.Root)
	if err != nil {
		return packet, err
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
// This is the only supported way to call a model from the app layer.
func Send(cfg Config, packet Packet) (string, error) {
	wire := packet.Wire
	if wire == "" {
		wire = packet.Prompt
	}
	return providers.Ask(cfg.Provider, wire, cfg.Model, cfg.Write)
}

// Run gathers then sends in one shot (CLI path). Always goes through Gather → Send.
func Run(cfg Config, prompt string) (string, Packet, error) {
	packet, err := Gather(cfg, prompt)
	if err != nil {
		return "", packet, err
	}
	text, err := Send(cfg, packet)
	return text, packet, err
}

// FormatReport prints a local evidence packet for CLI context commands.
func FormatReport(label, query string, mode Mode, r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s v1 — exact token/path matching only; no embeddings or dependency graph.\nquery: %s\nmode: %s\n\n", label, query, mode)
	for i, c := range r.Citations {
		fmt.Fprintf(&b, "%02d %s:%d-%d score %d\n%s\n\n", i+1, c.Path, c.StartLine, c.EndLine, c.Score, c.Snippet)
	}
	return b.String()
}

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

// StatusLine is a compact inspector string.
func StatusLine(cfg Config, packet Packet) string {
	mode := "off"
	if cfg.Super {
		mode = "on"
	}
	return fmt.Sprintf("super %s · %d cites · write %t", mode, packet.CiteCount, cfg.Write)
}
