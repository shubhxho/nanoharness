package harness

import "time"

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
	s.Evidence = Top(cites, s.Limit)
	if s.Limit <= 0 {
		s.Evidence = Top(cites, AttachLimit)
	}
}
