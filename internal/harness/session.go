package harness

import (
	"fmt"
	"strings"
	"time"

	"github.com/shubhxho/nanoharness/internal/terminal"
)

// Session is the app-facing Superpower handle. CLI and TUI should route all
// gather / search / send / login work through a Session instead of calling
// providers or context directly.
type Session struct {
	Config
}

// NewSession returns a Superpower session with provider defaults applied.
func NewSession(provider string) *Session {
	return &Session{Config: DefaultConfig(provider)}
}

// WithRoot sets the workspace root used for lexical gather/search.
func (s *Session) WithRoot(root string) *Session {
	s.Root = root
	return s
}

// WithModel sets the provider model id (empty keeps vendor default).
func (s *Session) WithModel(model string) *Session {
	s.Model = model
	return s
}

// WithWrite arms Codex workspace-write for the next send.
func (s *Session) WithWrite(write bool) *Session {
	s.Write = write
	return s
}

// WithSuper enables or disables automatic local citation gather.
func (s *Session) WithSuper(on bool) *Session {
	s.Super = on
	if on {
		s.Attach = true
	}
	return s
}

// WithAttach controls attaching preloaded or gathered citations.
func (s *Session) WithAttach(on bool) *Session {
	s.Attach = on
	return s
}

// WithEvidence replaces the preloaded citation set merged during Gather.
func (s *Session) WithEvidence(cites []Citation) *Session {
	s.Evidence = cites
	return s
}

// WithGoal sets a persistent Continual Harness goal (Prime Agent–style).
func (s *Session) WithGoal(goal string) *Session {
	s.Continual.Goal = strings.TrimSpace(goal)
	return s
}

// RememberNote appends a short evidence-backed memory to Continual state.
func (s *Session) RememberNote(note string) *Session {
	s.Continual.remember(note)
	return s
}

// WithAutonomous enables bounded autonomous execution (prime-agent gates/turns).
func (s *Session) WithAutonomous(on bool) *Session {
	s.Continual.Autonomous = on
	if on {
		s.Write = true // autonomous work needs tools on prime-agent
		s.Attach = true
	}
	return s
}

// WithGate adds a completion gate command for autonomous prime-agent runs.
func (s *Session) WithGate(cmd string) *Session {
	s.Continual.addGate(cmd)
	return s
}

// WithMaxTurns bounds autonomous assistant turns (default 12).
func (s *Session) WithMaxTurns(n int) *Session {
	if n > 0 {
		s.Continual.MaxTurns = n
	}
	return s
}

// ClearContinual resets goal, memories, gates, and autonomous flags.
func (s *Session) ClearContinual() *Session {
	s.Continual = Continual{}
	return s
}

// Gather builds a confirmable wire packet for prompt.
func (s *Session) Gather(prompt string) (Packet, error) {
	return Gather(s.Config, prompt)
}

// Send delivers a gathered packet through the provider transport.
func (s *Session) Send(packet Packet) (string, error) {
	return Send(s.Config, packet)
}

// Ask runs Gather then Send in one shot (CLI / non-interactive path).
func (s *Session) Ask(prompt string) (Result, error) {
	return Run(s.Config, prompt)
}

// AskTimed is Ask with gather/send durations for progress output.
func (s *Session) AskTimed(prompt string) (Result, time.Duration, time.Duration, error) {
	return RunTimed(s.Config, prompt)
}

// Search runs local lexical retrieval for the session root.
func (s *Session) Search(query string, mode Mode) (Report, error) {
	return Search(s.Root, query, mode)
}

// Index scans the session root once.
func (s *Session) Index() (Report, error) {
	return Index(s.Root)
}

// Remember stores citations on the session for later attach/merge.
func (s *Session) Remember(cites []Citation) {
	limit := s.Limit
	if limit <= 0 {
		limit = AttachLimit
	}
	s.Evidence = Top(cites, limit)
}

// Login refreshes credentials for a provider through the harness catalog.
func (s *Session) Login(kind string, apiKey bool) error {
	if kind == "" {
		kind = s.Provider
	}
	return Login(kind, apiKey)
}

// Auth reports whether the session provider is ready to send.
func (s *Session) Auth() string {
	return AuthStatus(s.Provider)
}

// Status is a multi-line diagnostic for CLI/TUI health checks.
func (s *Session) Status(version string) string {
	root := s.Root
	if root == "" {
		if cwd, err := resolveRoot(""); err == nil {
			root = cwd
		}
	}
	var b strings.Builder
	term := terminal.Detect()
	fmt.Fprintf(&b, "nanoharness %s\n", version)
	fmt.Fprintf(&b, "terminal  %s\n", term.Summary())
	fmt.Fprintf(&b, "root      %s\n", root)
	fmt.Fprintf(&b, "provider  %s\n", s.Provider)
	fmt.Fprintf(&b, "model     %s\n", displayOrDefault(s.Model))
	fmt.Fprintf(&b, "auth      %s\n", s.Auth())
	fmt.Fprintf(&b, "super     %t\n", s.Super)
	fmt.Fprintf(&b, "attach    %t\n", s.Attach)
	fmt.Fprintf(&b, "write     %t\n", s.Write)
	fmt.Fprintf(&b, "evidence  %d cites\n", len(s.Evidence))
	fmt.Fprintf(&b, "continual %s\n", ContinualSummary(s.Continual))
	b.WriteString(ContinualDetail(s.Continual))
	b.WriteString("providers\n")
	for _, p := range Profiles {
		fmt.Fprintf(&b, "  %-10s %s\n", p.ID, AuthStatus(p.ID))
	}
	return b.String()
}
