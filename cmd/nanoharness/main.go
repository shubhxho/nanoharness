package main

import (
	"fmt"
	"os"

	"github.com/shubhxho/nanoharness/internal/cli"
	"github.com/shubhxho/nanoharness/internal/tui"
	"github.com/shubhxho/nanoharness/internal/version"
)

// Set by GoReleaser / Makefile ldflags (-X main.version / -X main.commit).
// Also mirrored into internal/version for Session.Status and shared callers.
var (
	versionFlag = "dev"
	commitFlag  = ""
)

func syncVersion() {
	if versionFlag != "" && versionFlag != "dev" {
		version.Version = versionFlag
	}
	if commitFlag != "" {
		version.Commit = commitFlag
	}
	// Prefer makefile -X paths that set internal/version directly when present.
}

func main() {
	syncVersion()
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "tui" {
		tui.Version = version.Full()
		if err := tui.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	var err error
	switch args[0] {
	case "login":
		err = cli.Login(args[1:])
	case "run":
		err = cli.Run(args[1:])
	case "context":
		err = cli.Context(args[1:])
	case "status":
		err = cli.Status(version.Full(), args[1:])
	case "version", "--version", "-V":
		fmt.Println("nanoharness", version.Full())
		if rev := version.Rev(); rev != "" {
			fmt.Println("commit", rev)
		}
	case "help", "--help", "-h":
		usage()
	default:
		err = fmt.Errorf("unknown command: %s\n\nrun `nanoharness help`", args[0])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`nanoharness — Superpower terminal harness for Codex, Prime Agent, OpenAI, Anthropic, and pi

INSTALL:
  go install github.com/shubhxho/nanoharness/cmd/nanoharness@latest

USAGE:
  nanoharness                                              # TUI
  nanoharness status [--provider ID]
  nanoharness login <codex|prime|openai|anthropic|claude> [--api-key]
  nanoharness run [--provider ID] [--model ID] [--write] [--super|--no-super]
                 [--goal TEXT] [--auto] [--gate CMD] [--max-turns N] PROMPT
  nanoharness context <index|query|research|impact> TERMS
  nanoharness version

Providers: codex · prime (Prime Intellect prime-agent) · openai · anthropic · pi
Continual Harness: goals, memories, bounded autonomous gates (prime-agent).
Every ask runs through a harness Session: gather → confirm/send.
Build identity comes from git rev-parse / describe (see make build).`)
}
