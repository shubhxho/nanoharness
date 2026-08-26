package harness

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	local "github.com/shubhxho/nanoharness/internal/context"
)

func resolveRoot(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	return os.Getwd()
}

// ParseMode maps a CLI/TUI mode name to a retrieval mode.
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

// Index scans the tree once and reports bounds.
func Index(root string) (Report, error) {
	root, err := resolveRoot(root)
	if err != nil {
		return Report{}, err
	}
	return local.Search(root, "index")
}

// Normalize fills defaults and validates provider identity.
func (cfg Config) Normalize() (Config, error) {
	if cfg.Provider == "" {
		cfg.Provider = "codex"
	}
	if _, ok := Find(cfg.Provider); !ok {
		return cfg, fmt.Errorf("provider must be codex, prime, openai, anthropic, or pi")
	}
	if cfg.Limit <= 0 {
		cfg.Limit = AttachLimit
	}
	if cfg.Super {
		cfg.Attach = true
	}
	return cfg, nil
}

// Gather builds the wire prompt: optional local search + citation attachment.
func Gather(cfg Config, prompt string) (Packet, error) {
	cfg, err := cfg.Normalize()
	if err != nil {
		return Packet{}, err
	}
	prompt = strings.TrimSpace(prompt)
	packet := Packet{Prompt: prompt, Wire: prompt}
	if prompt == "" {
		return packet, fmt.Errorf("prompt is empty")
	}

	root, err := resolveRoot(cfg.Root)
	if err != nil {
		return packet, err
	}

	var fresh []Citation
	if cfg.Super {
		terms := local.ExtractTerms(prompt)
		packet.Terms = terms
		query := strings.Join(terms, " ")
		if query == "" {
			query = prompt
		}

		mode := ModeResearch
		if looksLikeSymbol(terms, prompt) {
			mode = ModeImpact
		}
		packet.Mode = mode

		report, err := local.SearchMode(root, query, mode)
		if err != nil {
			return packet, err
		}
		// If impact/symbol mode is thin, fall back to research and merge.
		if mode == ModeImpact && len(report.Citations) < 2 {
			soft, softErr := local.SearchMode(root, query, ModeResearch)
			if softErr != nil {
				return packet, softErr
			}
			report.Citations = local.MergeCitations(report.Citations, soft.Citations)
			report.ScannedBytes = max64(report.ScannedBytes, soft.ScannedBytes)
			report.Skipped += soft.Skipped
			report.Truncated = report.Truncated || soft.Truncated
			packet.Mode = ModeResearch
		}
		packet.Report = report
		packet.Gathered = true
		fresh = report.Citations
	}

	citations := local.Top(local.MergeCitations(cfg.Evidence, fresh), cfg.Limit)
	packet.Citations = citations
	packet.CiteCount = len(citations)

	prefix := cfg.Continual.preamble()
	base := prefix + prompt
	packet.Wire = base
	packet.Prompt = prompt

	attach := packet.CiteCount > 0 && (cfg.Super || cfg.Attach)
	if attach {
		packet.Wire = prefix + local.SuperPreamble(packet.CiteCount) + prompt + "\n\n" + local.Render(citations)
		packet.Confirm = true
	}
	if cfg.Write || cfg.Continual.Autonomous {
		packet.Confirm = true
	}
	return packet, nil
}

func looksLikeSymbol(terms []string, prompt string) bool {
	if len(terms) == 1 {
		t := terms[0]
		if strings.ContainsAny(t, "_-") || hasMixedCase(prompt, t) {
			return true
		}
		if len(t) >= 6 {
			return true
		}
	}
	trimmed := strings.TrimSpace(prompt)
	if len(terms) <= 2 && !strings.Contains(trimmed, " ") && len(trimmed) >= 3 {
		return true
	}
	return false
}

func hasMixedCase(prompt, term string) bool {
	for _, field := range strings.FieldsFunc(prompt, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-')
	}) {
		if strings.EqualFold(field, term) && field != strings.ToLower(field) {
			return true
		}
	}
	return false
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
