package harness

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shubhxho/nanoharness/internal/terminal"
)

// Session is the app-facing Superpower handle. CLI and TUI should route all
// gather / search / send / login work through a Session instead of calling
// providers or context directly.
type Session struct {
	Config
	Stats SessionStats
	Turns []Turn
}

// NewSession returns a Superpower session with provider defaults applied.
func NewSession(provider string) *Session {
	s := &Session{Config: DefaultConfig(provider)}
	if cwd, err := os.Getwd(); err == nil {
		s.Root = cwd
	}
	return s
}

// Reset clears continual state, evidence, write arm, and activity counters.
func (s *Session) Reset() *Session {
	s.ClearContinual()
	s.WithEvidence(nil)
	s.WithWrite(false)
	s.Stats = SessionStats{}
	s.Turns = nil
	return s
}

// ValidateSend checks provider auth before leaving the machine.
func (s *Session) ValidateSend() error {
	if _, err := s.Config.Normalize(); err != nil {
		return err
	}
	status := strings.ToLower(s.Auth())
	switch {
	case strings.Contains(status, "missing"),
		strings.Contains(status, "unavailable"),
		strings.Contains(status, "login needed"):
		return fmt.Errorf("provider %s not ready: %s (try: nanoharness login %s)", s.Provider, s.Auth(), s.Provider)
	}
	return nil
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

// NeedsConfirm reports whether a gathered packet must be approved before send.
func (s *Session) NeedsConfirm(packet Packet) bool {
	return packet.Confirm
}

// PipelineLine is a compact header/footer snapshot for TUI and CLI progress.
func (s *Session) PipelineLine() string {
	parts := []string{
		fmt.Sprintf("gathers %d", s.Stats.Gathers),
		fmt.Sprintf("sends %d", s.Stats.Sends),
		ContinualSummary(s.Continual),
	}
	if s.Stats.LastGather > 0 {
		parts = append(parts, "last gather "+s.Stats.LastGather.Round(time.Millisecond).String())
	}
	if s.Stats.LastSend > 0 {
		parts = append(parts, "last send "+s.Stats.LastSend.Round(time.Millisecond).String())
	}
	return strings.Join(parts, " · ")
}

// Gather builds a confirmable wire packet for prompt.
func (s *Session) Gather(prompt string) (Packet, error) {
	return Gather(s.Config, prompt)
}

// GatherPrepared runs Gather and merges fresh citations into session evidence.
func (s *Session) GatherPrepared(prompt string) (Packet, error) {
	packet, err := s.Gather(prompt)
	if err != nil {
		return packet, err
	}
	s.Stats.Gathers++
	if len(packet.Citations) > 0 {
		s.Remember(packet.Citations)
	}
	return packet, nil
}

// GatherTimed is GatherPrepared with elapsed time for progress UI.
func (s *Session) GatherTimed(prompt string) (Packet, time.Duration, error) {
	start := time.Now()
	packet, err := s.GatherPrepared(prompt)
	d := time.Since(start)
	s.Stats.LastGather = d
	return packet, d, err
}

// Send delivers a gathered packet through the provider transport.
func (s *Session) Send(packet Packet) (string, error) {
	if err := s.ValidateSend(); err != nil {
		return "", err
	}
	text, err := Send(s.Config, packet)
	s.Stats.Sends++
	if s.Write {
		s.WithWrite(false)
	}
	return text, err
}

// SendTimed is Send with elapsed time for progress UI.
func (s *Session) SendTimed(packet Packet) (string, time.Duration, error) {
	start := time.Now()
	text, err := s.Send(packet)
	d := time.Since(start)
	s.Stats.LastSend = d
	if err == nil {
		s.addTurn(packet, text, d, true)
	} else {
		s.addTurn(packet, err.Error(), d, false)
	}
	return text, d, err
}

// Pipeline runs GatherPrepared → Send in one shot (CLI / non-interactive path).
func (s *Session) Pipeline(prompt string) (Result, error) {
	result, _, _, err := s.PipelineTimed(prompt)
	return result, err
}

// Ask runs Pipeline (alias kept for CLI compatibility).
func (s *Session) Ask(prompt string) (Result, error) {
	return s.Pipeline(prompt)
}

// PipelineTimed is the full harness path with gather/send durations.
func (s *Session) PipelineTimed(prompt string) (Result, time.Duration, time.Duration, error) {
	packet, gatherFor, err := s.GatherTimed(prompt)
	if err != nil {
		return Result{Packet: packet, GatherFor: gatherFor}, gatherFor, 0, err
	}
	text, sendFor, err := s.SendTimed(packet)
	return Result{Text: text, Packet: packet, GatherFor: gatherFor, SendFor: sendFor}, gatherFor, sendFor, err
}

// AskTimed is PipelineTimed (alias kept for CLI compatibility).
func (s *Session) AskTimed(prompt string) (Result, time.Duration, time.Duration, error) {
	return s.PipelineTimed(prompt)
}

// Search runs local lexical retrieval for the session root.
func (s *Session) Search(query string, mode Mode) (Report, error) {
	return Search(s.Root, query, mode)
}

// SearchRemember runs Search and stores citations on the session.
func (s *Session) SearchRemember(query string, mode Mode) (Report, error) {
	report, err := s.Search(query, mode)
	if err != nil {
		return report, err
	}
	s.Remember(report.Citations)
	return report, nil
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
	fmt.Fprintf(&b, "pipeline  %s\n", s.PipelineLine())
	if t, ok := s.LastTurn(); ok {
		fmt.Fprintf(&b, "last turn %s\n", FormatLastTurn(t))
	}
	fmt.Fprintf(&b, "continual %s\n", ContinualSummary(s.Continual))
	b.WriteString(ContinualDetail(s.Continual))
	b.WriteString("providers\n")
	for _, p := range Profiles {
		fmt.Fprintf(&b, "  %-10s %s\n", p.ID, AuthStatus(p.ID))
	}
	return b.String()
}
