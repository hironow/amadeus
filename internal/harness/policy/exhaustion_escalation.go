package policy

import (
	"github.com/hironow/amadeus/internal/domain"
)

// ExhaustionAction represents what to do when retry budget is exhausted
// or provider state is unhealthy.
type ExhaustionAction int

const (
	// ExhaustionWait means the circuit breaker will block in Allow();
	// the caller should wait for the backoff to elapse.
	ExhaustionWait ExhaustionAction = iota
	// ExhaustionPause means the caller should enter a paused state
	// and log a prominent banner.
	ExhaustionPause
	// ExhaustionAbort means the caller should abort the current run
	// and emit an event.
	ExhaustionAbort
)

// EvaluateExhaustion determines the action based on provider state and budget.
//
// Decision table:
//
//	| state    | budget | action |
//	|----------|--------|--------|
//	| active   | 0      | Pause  |
//	| waiting  | 0      | Wait   |
//	| degraded | any    | Pause  |
//	| paused   | any    | Abort  |
func EvaluateExhaustion(snapshot domain.ProviderStateSnapshot) ExhaustionAction {
	switch snapshot.State {
	case domain.ProviderStatePaused:
		return ExhaustionAbort
	case domain.ProviderStateDegraded:
		return ExhaustionPause
	case domain.ProviderStateWaiting:
		return ExhaustionWait
	default:
		return ExhaustionPause
	}
}
