package domain

import "time"

// HandoverState captures in-progress work state when an operation is
// interrupted by a signal. The struct is pure data — no context, no I/O.
type HandoverState struct {
	Tool         string            // "amadeus"
	Operation    string            // "divergence"
	Timestamp    time.Time
	InProgress   string            // Current task description
	Completed    []string          // What was done
	Remaining    []string          // What's left
	PartialState map[string]string // Tool-specific state (key=label, value=detail)
}
