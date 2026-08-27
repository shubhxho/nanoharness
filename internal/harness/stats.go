package harness

import "time"

// SessionStats tracks activity routed through a Session.
type SessionStats struct {
	Gathers    int
	Sends      int
	LastGather time.Duration
	LastSend   time.Duration
}
