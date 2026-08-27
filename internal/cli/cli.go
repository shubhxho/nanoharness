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

type runOptions struct {
	provider string
	model    string
	root     string
	write    bool
	super    *bool
	goal     string
	auto     bool
	gate     []string
	maxTurns int
}

// Run executes a Superpower-aware provider ask through a harness Session.
func Run(args []string) error {
	opts := runOptions{provider: "codex"}
	var words []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider":
			i++
			if i < len(args) {
				opts.provider = args[i]
			}
		case "--model":
			i++
			if i < len(args) {
				opts.model = args[i]
			}
		case "--root":
			i++
			if i < len(args) {
				opts.root = args[i]
			}
		case "--write":
			opts.write = true
		case "--super":
			v := true
			opts.super = &v
		case "--no-super":
			v := false
			opts.super = &v
		case "--goal":
			i++
			if i < len(args) {
				opts.goal = args[i]
			}
		case "--auto", "--autonomous":
			opts.auto = true
		case "--gate":
			i++
			if i < len(args) {
				opts.gate = append(opts.gate, args[i])
			}
		case "--max-turns":
			i++
			if i < len(args) {
				n, _ := strconv.Atoi(args[i])
				opts.maxTurns = n
			}
		default:
			words = append(words, args[i])
		}
	}
	prompt := strings.Join(words, " ")
	if strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("run needs a prompt; example: nanoharness run --provider prime --goal \"ship fix\" \"implement the change\"")
	}

	session := harness.NewSession(opts.provider)
	if opts.root != "" {
		session.WithRoot(opts.root)
	}
	if opts.model != "" {
		session.WithModel(opts.model)
	}
	if opts.write {
		session.WithWrite(true)
	}
	if opts.super != nil {
		session.WithSuper(*opts.super)
		if !*opts.super {
			session.WithAttach(false)
		}
	}
	if opts.goal != "" {
		session.WithGoal(opts.goal)
	}
	if opts.auto {
		session.WithAutonomous(true)
	}
	for _, g := range opts.gate {
		session.WithGate(g)
	}
	if opts.maxTurns > 0 {
		session.WithMaxTurns(opts.maxTurns)
	}

	fmt.Fprintln(os.Stderr, "# harness: gather…")
	fmt.Fprintf(os.Stderr, "# terminal: %s · %s\n", terminal.Detect().Summary(), session.PipelineLine())
	result, gatherFor, sendFor, err := session.PipelineTimed(prompt)
	if err != nil {
		if gatherFor > 0 && result.Packet.Gathered {
			fmt.Fprintf(os.Stderr, "# harness: gather failed after %s: %v\n", gatherFor.Round(time.Millisecond), err)
		}
		return err
	}
	fmt.Fprintf(os.Stderr, "# harness: %s · gather %s\n", harness.Describe(result.Packet), gatherFor.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "# harness: sent via %s in %s · %s\n", session.Provider, sendFor.Round(time.Millisecond), session.PipelineLine())
	if t, ok := session.LastTurn(); ok {
		fmt.Fprintf(os.Stderr, "# last turn: %s\n", harness.FormatLastTurn(t))
	}
	fmt.Println(result.Text)
	return nil
}

// Context runs local lexical retrieval through a harness Session.
func Context(args []string) error {
	session := harness.NewSession("codex").WithSuper(false)
	remember := false
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--remember":
			remember = true
		case "--root":
			i++
			if i < len(args) {
				session.WithRoot(args[i])
			}
		default:
			positional = append(positional, args[i])
		}
	}
	if len(positional) == 0 {
		return fmt.Errorf("context needs a mode")
	}
	mode := positional[0]
	if mode == "index" {
		root, _ := os.Getwd()
		if session.Root != "" {
			root = session.Root
		}
		r, err := session.Index()
		if err == nil {
			fmt.Printf("LOCAL LEXICAL CONTEXT v1\nroot: %s\nscanned: %d bytes · skipped: %d\n", root, r.ScannedBytes, r.Skipped)
		}
		return err
	}
	query := strings.Join(positional[1:], " ")
	if query == "" {
		return fmt.Errorf("context %s needs terms", mode)
	}
	searchMode, err := harness.ParseMode(mode)
	if err != nil {
		return fmt.Errorf("context mode must be index, query, research, or impact")
	}
	var r harness.Report
	if remember {
		r, err = session.SearchRemember(query, searchMode)
		if err == nil {
			fmt.Fprintf(os.Stderr, "# harness: remembered %d citations on session\n", len(session.Evidence))
		}
	} else {
		r, err = session.Search(query, searchMode)
	}
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
		if args[i] == "--root" && i+1 < len(args) {
			session.WithRoot(args[i+1])
			i++
		}
	}
	fmt.Print(session.Status(version))
	return nil
}
