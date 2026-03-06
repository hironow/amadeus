package domain

import (
	"time"
)

// CheckAggregate encapsulates the domain logic for amadeus check operations.
// It owns the check count, force-full-next flag, and previous result state,
// enforcing invariants and producing events as return values (no side effects).
type CheckAggregate struct {
	config        Config
	checkCount    int
	forceFullNext bool
}

// NewCheckAggregate creates a new CheckAggregate with the given config.
func NewCheckAggregate(cfg Config) *CheckAggregate {
	return &CheckAggregate{config: cfg}
}

// Restore hydrates the aggregate from a persisted CheckResult projection.
func (a *CheckAggregate) Restore(result CheckResult) {
	a.checkCount = result.CheckCountSinceFull
	a.forceFullNext = result.ForceFullNext
}

// CheckCount returns the current check count since last full check.
func (a *CheckAggregate) CheckCount() int {
	return a.checkCount
}

// ForceFullNext returns whether the next check is forced to be full.
func (a *CheckAggregate) ForceFullNext() bool {
	return a.forceFullNext
}

// SetForceFullNext sets the force-full-next flag.
func (a *CheckAggregate) SetForceFullNext(v bool) {
	a.forceFullNext = v
}

// ShouldFullCheck determines whether the next check should be a full scan.
// Returns true if forceFlag is set, ForceFullNext is true, or the check count
// has reached the configured interval.
func (a *CheckAggregate) ShouldFullCheck(forceFlag bool) bool {
	if forceFlag || a.forceFullNext {
		return true
	}
	return a.checkCount >= a.config.FullCheck.Interval
}

// AdvanceCheckCount updates the internal check counter.
// If fullCheck is true, the counter resets to 0; otherwise it increments by 1.
func (a *CheckAggregate) AdvanceCheckCount(fullCheck bool) {
	if fullCheck {
		a.checkCount = 0
	} else {
		a.checkCount++
	}
}

// ShouldPromoteToFull returns true when the absolute divergence change between
// the previous and current values exceeds the configured on_divergence_jump threshold.
func (a *CheckAggregate) ShouldPromoteToFull(previousDivergence, currentDivergence float64) bool {
	delta := currentDivergence - previousDivergence
	if delta < 0 {
		delta = -delta
	}
	return delta >= a.config.FullCheck.OnDivergenceJump
}

// RecordInboxConsumed produces an inbox.consumed event.
func (a *CheckAggregate) RecordInboxConsumed(data InboxConsumedData, now time.Time) (Event, error) {
	return NewEvent(EventInboxConsumed, data, now)
}

// RecordForceFullNextSet produces a force_full_next.set event and sets the flag.
func (a *CheckAggregate) RecordForceFullNextSet(prevDiv, currDiv float64, now time.Time) (Event, error) {
	a.forceFullNext = true
	return NewEvent(EventForceFullNextSet, ForceFullNextSetData{
		PreviousDivergence: prevDiv,
		CurrentDivergence:  currDiv,
	}, now)
}

// RecordDMailGenerated produces a dmail.generated event.
func (a *CheckAggregate) RecordDMailGenerated(dmail DMail, now time.Time) (Event, error) {
	return NewEvent(EventDMailGenerated, DMailGeneratedData{DMail: dmail}, now)
}

// RecordConvergenceDetected produces a convergence.detected event.
func (a *CheckAggregate) RecordConvergenceDetected(alert ConvergenceAlert, now time.Time) (Event, error) {
	return NewEvent(EventConvergenceDetected, ConvergenceDetectedData{Alert: alert}, now)
}

// RecordDMailCommented produces a dmail.commented event.
func (a *CheckAggregate) RecordDMailCommented(dmailName, issueID string, now time.Time) (Event, error) {
	return NewEvent(EventDMailCommented, DMailCommentedData{
		DMail: dmailName, IssueID: issueID,
	}, now)
}

// RecordCheck produces events for a completed check result.
// For full checks, it also produces a baseline.updated event.
// The caller is responsible for persisting the returned events.
func (a *CheckAggregate) RecordCheck(result CheckResult, now time.Time) ([]Event, error) {
	result.CheckCountSinceFull = a.checkCount
	result.ForceFullNext = a.forceFullNext

	checkEv, err := NewEvent(EventCheckCompleted, CheckCompletedData{Result: result}, now)
	if err != nil {
		return nil, err
	}
	events := []Event{checkEv}

	if result.Type == CheckTypeFull && !result.GateDenied {
		baselineEv, bErr := NewEvent(EventBaselineUpdated, BaselineUpdatedData{
			Commit: result.Commit, Divergence: result.Divergence,
		}, now)
		if bErr != nil {
			return events, bErr
		}
		events = append(events, baselineEv)
	}

	return events, nil
}
