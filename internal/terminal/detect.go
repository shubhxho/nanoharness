// Package terminal detects the host emulator so the TUI can adapt rendering.
package terminal

import (
	"os"
	"strings"
)

// Info describes the terminal nanoharness is running inside.
type Info struct {
	Program   string
	Version   string
	Term      string
	ColorTerm string
	Ghostty   bool
	TrueColor bool
}

// Detect reads standard terminal environment variables.
func Detect() Info {
	info := Info{
		Program:   os.Getenv("TERM_PROGRAM"),
		Version:   os.Getenv("TERM_PROGRAM_VERSION"),
		Term:      os.Getenv("TERM"),
		ColorTerm: os.Getenv("COLORTERM"),
	}
	info.Ghostty = strings.EqualFold(info.Program, "ghostty") ||
		strings.Contains(strings.ToLower(info.Term), "ghostty")
	info.TrueColor = strings.EqualFold(info.ColorTerm, "truecolor") ||
		strings.EqualFold(info.ColorTerm, "24bit")
	return info
}

// Label returns a compact host name for headers and status output.
func (i Info) Label() string {
	if i.Ghostty {
		if i.Version != "" {
			return "ghostty " + i.Version
		}
		return "ghostty"
	}
	if i.Program != "" {
		if i.Version != "" {
			return i.Program + " " + i.Version
		}
		return i.Program
	}
	if i.Term != "" {
		return i.Term
	}
	return "unknown"
}

// Summary is a one-line status string for CLI /status and the inspector.
func (i Info) Summary() string {
	parts := []string{i.Label()}
	if i.TrueColor {
		parts = append(parts, "truecolor")
	}
	return strings.Join(parts, " · ")
}
