package providers

// AskOptions carries Continual-Harness-style controls into a provider call.
// Inspired by Prime Agent's goal + bounded autonomous execution model; this is
// an independent nanoharness surface, not a copy of Prime Agent internals.
type AskOptions struct {
	Write      bool
	Root       string
	Goal       string
	Autonomous bool
	Gates      []string
	MaxTurns   int
}
