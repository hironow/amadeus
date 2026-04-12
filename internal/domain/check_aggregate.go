package domain

import (
	"time"
)

// defaultForceFullCooldown is the number of check cycles to skip after a
// force-full-next check completes, preventing rapid consecutive full checks.
const defaultForceFullCooldown = 3

// AggregateTypeCheck is the aggregate type for integrity check events.
const AggregateTypeCheck = "check"

// CheckAggregate encapsulates the domain logic for amadeus check operations.
// It owns the check count, force-full-next flag, and previous result state,
// enforcing invariants and producing events as return values (no side effects).
type CheckAggregate struct {
	checkID           string
	config            Config
	checkCount        int
	forceFullNext     bool
	cooldownRemaining int
	seqNr             uint64
}

// NewCheckAggregate creates a new CheckAggregate with the given config.
func NewCheckAggregate(cfg Config) *CheckAggregate {
	return &CheckAggregate{config: cfg}
}

// SetCheckID sets the check ID (used for event correlation).
func (a *CheckAggregate) SetCheckID(id string) {
	a.checkID = id
}

// CheckID returns the current check ID.
func (a *CheckAggregate) CheckID() string {
	return a.checkID
}

// Restore hydrates the aggregate from a persisted CheckResult projection.
func (a *CheckAggregate) Restore(result CheckResult) {
	a.checkCount = result.CheckCountSinceFull
	a.forceFullNext = result.ForceFullNext
	a.cooldownRemaining = result.CooldownRemaining
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
// Returns true if forceFlag is set (explicit --full always wins), ForceFullNext
// is true, or the check count has reached the configured interval.
// Cooldown suppresses ForceFullNext and interval-based triggers but never
// overrides an explicit forceFlag.
func (a *CheckAggregate) ShouldFullCheck(forceFlag bool) bool {
	if forceFlag {
		return true
	}
	if a.cooldownRemaining > 0 {
		return false
	}
	if a.forceFullNext {
		return true
	}
	return a.checkCount >= a.config.FullCheck.Interval
}

// CooldownRemaining returns the remaining cooldown cycles.
func (a *CheckAggregate) CooldownRemaining() int {
	return a.cooldownRemaining
}

// AdvanceCheckCount updates the internal check counter.
// If fullCheck is true, the counter resets to 0 and starts cooldown when
// wasForced is true (indicating the full check was triggered by forceFullNext).
// Otherwise it increments by 1 and decrements cooldown.
// The wasForced parameter is necessary because forceFullNext may have been
// cleared earlier in the pipeline (e.g. by detectShift) before this call.
func (a *CheckAggregate) AdvanceCheckCount(fullCheck bool, wasForced bool) {
	if fullCheck {
		a.checkCount = 0
		if wasForced || a.forceFullNext {
			a.cooldownRemaining = defaultForceFullCooldown
			a.forceFullNext = false
		}
	} else {
		a.checkCount++
		if a.cooldownRemaining > 0 {
			a.cooldownRemaining--
		}
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

// nextEvent creates an event tagged with check aggregate identity.
func (a *CheckAggregate) nextEvent(eventType EventType, data any, now time.Time) (Event, error) {
	a.seqNr++
	ev, err := NewEvent(eventType, data, now)
	if err != nil {
		return ev, err
	}
	ev.AggregateID = a.checkID
	ev.AggregateType = AggregateTypeCheck
	ev.SeqNr = a.seqNr
	return ev, nil
}

// RecordInboxConsumed produces an inbox.consumed event.
func (a *CheckAggregate) RecordInboxConsumed(data InboxConsumedData, now time.Time) (Event, error) {
	return a.nextEvent(EventInboxConsumed, data, now)
}

// RecordForceFullNextSet produces a force.full.next.set event and sets the flag.
func (a *CheckAggregate) RecordForceFullNextSet(prevDiv, currDiv float64, now time.Time) (Event, error) {
	a.forceFullNext = true
	return a.nextEvent(EventForceFullNextSet, ForceFullNextSetData{
		PreviousDivergence: prevDiv,
		CurrentDivergence:  currDiv,
	}, now)
}

// RecordDMailGenerated produces a dmail.generated event.
func (a *CheckAggregate) RecordDMailGenerated(dmail DMail, now time.Time) (Event, error) {
	return a.nextEvent(EventDMailGenerated, DMailGeneratedData{DMail: dmail}, now)
}

// RecordConvergenceDetected produces a convergence.detected event.
func (a *CheckAggregate) RecordConvergenceDetected(alert ConvergenceAlert, now time.Time) (Event, error) {
	return a.nextEvent(EventConvergenceDetected, ConvergenceDetectedData{Alert: alert}, now)
}

// RecordDMailCommented produces a dmail.commented event.
func (a *CheckAggregate) RecordDMailCommented(dmailName, issueID string, now time.Time) (Event, error) {
	return a.nextEvent(EventDMailCommented, DMailCommentedData{
		DMail: dmailName, IssueID: issueID,
	}, now)
}

// RecordRunStarted produces a run.started event.
func (a *CheckAggregate) RecordRunStarted(data RunStartedData, now time.Time) (Event, error) {
	return a.nextEvent(EventRunStarted, data, now)
}

// RecordRunStopped produces a run.stopped event.
func (a *CheckAggregate) RecordRunStopped(data RunStoppedData, now time.Time) (Event, error) {
	return a.nextEvent(EventRunStopped, data, now)
}

// RecordPRConvergenceChecked produces a pr.convergence.checked event.
func (a *CheckAggregate) RecordPRConvergenceChecked(data PRConvergenceCheckedData, now time.Time) (Event, error) {
	return a.nextEvent(EventPRConvergenceChecked, data, now)
}

// RecordPRMerged produces a pr.merged event.
func (a *CheckAggregate) RecordPRMerged(data PRMergedData, now time.Time) (Event, error) {
	return a.nextEvent(EventPRMerged, data, now)
}

// RecordPRMergeSkipped produces a pr.merge.skipped event.
func (a *CheckAggregate) RecordPRMergeSkipped(data PRMergeSkippedData, now time.Time) (Event, error) {
	return a.nextEvent(EventPRMergeSkipped, data, now)
}

// RecordCheck produces events for a completed check result.
// For full checks, it also produces a baseline.updated event.
// The caller is responsible for persisting the returned events.
func (a *CheckAggregate) RecordCheck(result CheckResult, now time.Time) ([]Event, error) {
	result.CheckCountSinceFull = a.checkCount
	result.ForceFullNext = a.forceFullNext
	result.CooldownRemaining = a.cooldownRemaining
	a.forceFullNext = false

	checkEv, err := a.nextEvent(EventCheckCompleted, CheckCompletedData{Result: result}, now)
	if err != nil {
		return nil, err
	}
	events := []Event{checkEv}

	if result.Type == CheckTypeFull && !result.GateDenied {
		baselineEv, bErr := a.nextEvent(EventBaselineUpdated, BaselineUpdatedData{
			Commit: result.Commit, Divergence: result.Divergence,
		}, now)
		if bErr != nil {
			return events, bErr
		}
		events = append(events, baselineEv)
	}

	return events, nil
}
