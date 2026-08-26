package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/shubhxho/nanoharness/internal/cli"
	"github.com/shubhxho/nanoharness/internal/tui"
)

// version is overwritten by GoReleaser ldflags; go install falls back to module version.
var version = "dev"

func resolveVersion() string {
	if version != "" && version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "tui" {
		tui.Version = resolveVersion()
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
	case "version", "--version", "-V":
		fmt.Println("nanoharness", resolveVersion())
	case "help", "--help", "-h":
		usage()
	default:
		err = fmt.Errorf("unknown command: %s", args[0])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`nanoharness — Superpower terminal harness

INSTALL:
  go install github.com/shubhxho/nanoharness/cmd/nanoharness@latest

USAGE:
  nanoharness
  nanoharness login <codex|openai|anthropic|claude> [--api-key]
  nanoharness run [--provider ID] [--model ID] [--write] [--super|--no-super] PROMPT
  nanoharness context <index|query|research|impact> TERMS
  nanoharness version

Everything runs through internal/harness (gather → confirm/send).`)
}
