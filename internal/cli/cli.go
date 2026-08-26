// Package cli implements nanoharness subcommands. All provider and context
// work goes through internal/harness.
package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shubhxho/nanoharness/internal/harness"
)

// Run executes a Superpower-aware provider ask: gather then send.
func Run(args []string) error {
	cfg := harness.DefaultConfig("codex")
	var words []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider":
			i++
			if i < len(args) {
				cfg.Provider = args[i]
			}
		case "--model":
			i++
			if i < len(args) {
				cfg.Model = args[i]
			}
		case "--write":
			cfg.Write = true
		case "--super":
			cfg.Super = true
			cfg.Attach = true
		case "--no-super":
			cfg.Super = false
			cfg.Attach = false
		default:
			words = append(words, args[i])
		}
	}
	prompt := strings.Join(words, " ")

	fmt.Fprintln(os.Stderr, "# harness: gather…")
	result, gatherFor, sendFor, err := harness.RunTimed(cfg, prompt)
	if err != nil {
		if gatherFor > 0 && result.Packet.Gathered {
			fmt.Fprintf(os.Stderr, "# harness: gather failed after %s: %v\n", gatherFor.Round(time.Millisecond), err)
		}
		return err
	}
	fmt.Fprintf(os.Stderr, "# harness: %s · gather %s\n", harness.Describe(result.Packet), gatherFor.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "# harness: sent via %s in %s\n", cfg.Provider, sendFor.Round(time.Millisecond))
	fmt.Println(result.Text)
	return nil
}

// Context runs local lexical retrieval through the harness.
func Context(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("context needs a mode")
	}
	mode := args[0]
	if mode == "index" {
		root, _ := os.Getwd()
		r, err := harness.Index("")
		if err == nil {
			fmt.Printf("LOCAL LEXICAL CONTEXT v1\nroot: %s\nscanned: %d bytes · skipped: %d\n", root, r.ScannedBytes, r.Skipped)
		}
		return err
	}
	query := strings.Join(args[1:], " ")
	if query == "" {
		return fmt.Errorf("context %s needs terms", mode)
	}
	searchMode, err := harness.ParseMode(mode)
	if err != nil {
		return fmt.Errorf("context mode must be index, query, research, or impact")
	}
	r, err := harness.Search("", query, searchMode)
	if err != nil {
		return err
	}
	fmt.Print(harness.FormatReport(harness.ModeLabel(searchMode), query, searchMode, r))
	return nil
}

// Login stores or refreshes provider credentials through the harness.
func Login(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("login needs a provider")
	}
	apiKey := len(args) > 1 && args[1] == "--api-key"
	return harness.Login(args[0], apiKey)
}
