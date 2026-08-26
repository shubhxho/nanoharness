// Package cli implements nanoharness subcommands. All provider and context
// work goes through an internal/harness.Session.
package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shubhxho/nanoharness/internal/harness"
	"github.com/shubhxho/nanoharness/internal/terminal"
)

// Run executes a Superpower-aware provider ask through a harness Session.
func Run(args []string) error {
	session := harness.NewSession("codex")
	var words []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider":
			i++
			if i < len(args) {
				session.Provider = args[i]
			}
		case "--model":
			i++
			if i < len(args) {
				session.WithModel(args[i])
			}
		case "--write":
			session.WithWrite(true)
		case "--super":
			session.WithSuper(true)
		case "--no-super":
			session.WithSuper(false).WithAttach(false)
		case "--goal":
			i++
			if i < len(args) {
				session.WithGoal(args[i])
			}
		case "--auto", "--autonomous":
			session.WithAutonomous(true)
		case "--gate":
			i++
			if i < len(args) {
				session.WithGate(args[i])
			}
		case "--max-turns":
			i++
			if i < len(args) {
				n, _ := strconv.Atoi(args[i])
				session.WithMaxTurns(n)
			}
		default:
			words = append(words, args[i])
		}
	}
	prompt := strings.Join(words, " ")
	if strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("run needs a prompt; example: nanoharness run --provider prime --goal \"ship fix\" \"implement the change\"")
	}

	fmt.Fprintln(os.Stderr, "# harness: gather…")
	fmt.Fprintf(os.Stderr, "# terminal: %s · %s\n", terminal.Detect().Summary(), harness.ContinualSummary(session.Continual))
	result, gatherFor, sendFor, err := session.AskTimed(prompt)
	if err != nil {
		if gatherFor > 0 && result.Packet.Gathered {
			fmt.Fprintf(os.Stderr, "# harness: gather failed after %s: %v\n", gatherFor.Round(time.Millisecond), err)
		}
		return err
	}
	fmt.Fprintf(os.Stderr, "# harness: %s · gather %s\n", harness.Describe(result.Packet), gatherFor.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "# harness: sent via %s in %s\n", session.Provider, sendFor.Round(time.Millisecond))
	fmt.Println(result.Text)
	return nil
}

// Context runs local lexical retrieval through a harness Session.
func Context(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("context needs a mode")
	}
	session := harness.NewSession("codex").WithSuper(false)
	mode := args[0]
	if mode == "index" {
		root, _ := os.Getwd()
		r, err := session.Index()
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
	r, err := session.Search(query, searchMode)
	if err != nil {
		return err
	}
	fmt.Print(harness.FormatReport(harness.ModeLabel(searchMode), query, searchMode, r))
	return nil
}

// Login stores or refreshes provider credentials through a harness Session.
func Login(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("login needs a provider")
	}
	apiKey := len(args) > 1 && args[1] == "--api-key"
	return harness.NewSession(args[0]).Login(args[0], apiKey)
}

// Status prints Session health for every provider.
func Status(version string, args []string) error {
	session := harness.NewSession("codex")
	for i := 0; i < len(args); i++ {
		if args[i] == "--provider" && i+1 < len(args) {
			session.Provider = args[i+1]
			i++
		}
	}
	fmt.Print(session.Status(version))
	return nil
}
