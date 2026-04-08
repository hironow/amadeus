package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// checkEventEmitter implements port.CheckEventEmitter.
// It wraps the aggregate + event store + projector + policy dispatcher.
// Emit chain: agg.Record*() → store.Append() → projector.Apply() → dispatch (best-effort).
type checkEventEmitter struct {
	agg           *domain.CheckAggregate
	store         port.EventStore
	seqAlloc      port.SeqAllocator // SQLite-backed global SeqNr (ADR S0040)
	seqNrFallback uint64            // fallback counter when seqAlloc is nil
	projector     domain.EventApplier
	dispatcher    port.EventDispatcher
	logger        domain.Logger
}

// NewCheckEventEmitter creates a CheckEventEmitter that wraps the aggregate event chain.
// seqAlloc is optional — pass nil to skip global SeqNr assignment.
func NewCheckEventEmitter(
	agg *domain.CheckAggregate,
	store port.EventStore,
	projector domain.EventApplier,
	dispatcher port.EventDispatcher,
	seqAlloc port.SeqAllocator,
	logger domain.Logger,
) port.CheckEventEmitter {
	return &checkEventEmitter{
		agg:        agg,
		store:      store,
		seqAlloc:   seqAlloc,
		projector:  projector,
		dispatcher: dispatcher,
		logger:     logger,
	}
}

// emit persists events and applies projections, then dispatches (best-effort).
// At least one of store or projector must be non-nil to prevent silent data loss.
func (e *checkEventEmitter) emit(ctx context.Context, events ...domain.Event) error {
	if e.store == nil && e.projector == nil {
		return fmt.Errorf("emit: neither EventStore nor Projector is configured — state would not be persisted")
	}
	// Tag events with aggregate identity and global SeqNr
	for i := range events {
		events[i].AggregateType = domain.AggregateTypeCheck
		if e.seqAlloc != nil {
			seq, err := e.seqAlloc.AllocSeqNr(ctx)
			if err != nil {
				return fmt.Errorf("alloc seq nr: %w", err)
			}
			events[i].SeqNr = seq
		} else {
			// Fallback: process-local counter for backward compatibility
			// until SeqAllocator is wired in all call sites.
			e.seqNrFallback++
			events[i].SeqNr = e.seqNrFallback
		}
	}
	if e.store != nil {
		if _, err := e.store.Append(ctx, events...); err != nil {
			return fmt.Errorf("append events: %w", err)
		}
	}
	if e.projector != nil {
		for _, ev := range events {
			if err := e.projector.Apply(ev); err != nil {
				return fmt.Errorf("apply event %s: %w", ev.Type, err)
			}
		}
	}
	if e.dispatcher != nil {
		for _, ev := range events {
			if err := e.dispatcher.Dispatch(ctx, ev); err != nil {
				e.logger.Warn("policy dispatch %s: %v", ev.Type, err)
			}
		}
	}
	return nil
}

func (e *checkEventEmitter) EmitInboxConsumed(ctx context.Context, data domain.InboxConsumedData, now time.Time) error {
	ev, err := e.agg.RecordInboxConsumed(data, now)
	if err != nil {
		return err
	}
	return e.emit(ctx, ev)
}

func (e *checkEventEmitter) EmitForceFullNextSet(ctx context.Context, prevDiv, currDiv float64, now time.Time) error {
	ev, err := e.agg.RecordForceFullNextSet(prevDiv, currDiv, now)
	if err != nil {
		return err
	}
	return e.emit(ctx, ev)
}

func (e *checkEventEmitter) EmitDMailGenerated(ctx context.Context, dmail domain.DMail, now time.Time) error {
	ev, err := e.agg.RecordDMailGenerated(dmail, now)
	if err != nil {
		return err
	}
	return e.emit(ctx, ev)
}

func (e *checkEventEmitter) EmitConvergenceDetected(ctx context.Context, alert domain.ConvergenceAlert, now time.Time) error {
	ev, err := e.agg.RecordConvergenceDetected(alert, now)
	if err != nil {
		return err
	}
	return e.emit(ctx, ev)
}

func (e *checkEventEmitter) EmitDMailCommented(ctx context.Context, dmailName, issueID string, now time.Time) error {
	ev, err := e.agg.RecordDMailCommented(dmailName, issueID, now)
	if err != nil {
		return err
	}
	return e.emit(ctx, ev)
}

func (e *checkEventEmitter) EmitCheck(ctx context.Context, result domain.CheckResult, now time.Time) error {
	events, err := e.agg.RecordCheck(result, now)
	if err != nil {
		return err
	}
	return e.emit(ctx, events...)
}

func (e *checkEventEmitter) EmitRunStarted(ctx context.Context, data domain.RunStartedData, now time.Time) error {
	ev, err := e.agg.RecordRunStarted(data, now)
	if err != nil {
		return err
	}
	return e.emit(ctx, ev)
}

func (e *checkEventEmitter) EmitRunStopped(ctx context.Context, data domain.RunStoppedData, now time.Time) error {
	ev, err := e.agg.RecordRunStopped(data, now)
	if err != nil {
		return err
	}
	return e.emit(ctx, ev)
}

func (e *checkEventEmitter) EmitPRConvergenceChecked(ctx context.Context, data domain.PRConvergenceCheckedData, now time.Time) error {
	ev, err := e.agg.RecordPRConvergenceChecked(data, now)
	if err != nil {
		return err
	}
	return e.emit(ctx, ev)
}

func (e *checkEventEmitter) EmitPRMerged(ctx context.Context, data domain.PRMergedData, now time.Time) error {
	ev, err := e.agg.RecordPRMerged(data, now)
	if err != nil {
		return err
	}
	return e.emit(ctx, ev)
}

func (e *checkEventEmitter) EmitPRMergeSkipped(ctx context.Context, data domain.PRMergeSkippedData, now time.Time) error {
	ev, err := e.agg.RecordPRMergeSkipped(data, now)
	if err != nil {
		return err
	}
	return e.emit(ctx, ev)
}

// checkStateProvider implements port.CheckStateProvider.
// It delegates to the aggregate for state queries and mutations.
type checkStateProvider struct {
	agg *domain.CheckAggregate
}

// NewCheckStateProvider creates a CheckStateProvider wrapping the aggregate.
func NewCheckStateProvider(agg *domain.CheckAggregate) port.CheckStateProvider {
	return &checkStateProvider{agg: agg}
}

func (m *checkStateProvider) ShouldFullCheck(forceFlag bool) bool {
	return m.agg.ShouldFullCheck(forceFlag)
}

func (m *checkStateProvider) ForceFullNext() bool {
	return m.agg.ForceFullNext()
}

func (m *checkStateProvider) SetForceFullNext(v bool) {
	m.agg.SetForceFullNext(v)
}

func (m *checkStateProvider) ShouldPromoteToFull(prevDiv, currDiv float64) bool {
	return m.agg.ShouldPromoteToFull(prevDiv, currDiv)
}

func (m *checkStateProvider) AdvanceCheckCount(fullCheck bool, wasForced bool) {
	m.agg.AdvanceCheckCount(fullCheck, wasForced)
}

func (m *checkStateProvider) Restore(result domain.CheckResult) {
	m.agg.Restore(result)
}
