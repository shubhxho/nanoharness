// Package harness is the single Superpower choke point for nanoharness.
// TUI and CLI gather, search, login, and send only through this package.
package harness

import (
	local "github.com/shubhxho/nanoharness/internal/context"
	"github.com/shubhxho/nanoharness/internal/providers"
)

// Config drives a single Superpower-aware request.
type Config struct {
	Super     bool
	Root      string
	Provider  string
	Model     string
	Write     bool
	Evidence  []Citation
	Attach    bool
	Limit     int
	Continual Continual
}

// Packet is a prompt ready to send (or confirm) through the harness.
type Packet struct {
	Prompt    string
	Wire      string
	Citations []Citation
	Report    Report
	Confirm   bool
	CiteCount int
	Terms     []string
	Gathered  bool
	Mode      Mode // primary retrieval mode used during gather
}

// Result is the outcome of a full Gather → Send pipeline run.
type Result struct {
	Text   string
	Packet Packet
}

// Re-exported types so app layers need not import context or providers.
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

// DefaultConfig returns Superpower-on defaults for the given provider.
func DefaultConfig(provider string) Config {
	if provider == "" {
		provider = "codex"
	}
	profile, ok := providers.Find(provider)
	model := ""
	if ok {
		model = profile.Default
	}
	return Config{
		Super:    true,
		Provider: provider,
		Model:    model,
		Attach:   true,
		Limit:    AttachLimit,
	}
}
