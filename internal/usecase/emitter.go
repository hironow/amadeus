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
	projector     port.ContextEventApplier
	dispatcher    port.EventDispatcher
	logger        domain.Logger
	checkID       string // enriches events with correlation metadata
	prevID        string // previous event ID for causation chain
	ctx           context.Context //nolint:containedctx // stored for trace propagation into emit chain
}

// NewCheckEventEmitter creates a CheckEventEmitter that wraps the aggregate event chain.
// seqAlloc is optional — pass nil to skip global SeqNr assignment.
func NewCheckEventEmitter(
	ctx context.Context,
	agg *domain.CheckAggregate,
	store port.EventStore,
	projector port.ContextEventApplier,
	dispatcher port.EventDispatcher,
	seqAlloc port.SeqAllocator,
	logger domain.Logger,
	checkID string,
) port.CheckEventEmitter {
	return &checkEventEmitter{
		agg:        agg,
		store:      store,
		seqAlloc:   seqAlloc,
		projector:  projector,
		dispatcher: dispatcher,
		logger:     logger,
		checkID:    checkID,
		ctx:        ctx,
	}
}

// emit persists events and applies projections, then dispatches (best-effort).
// At least one of store or projector must be non-nil to prevent silent data loss.
func (e *checkEventEmitter) emit(events ...domain.Event) error {
	ctx := e.ctx
	if e.store == nil && e.projector == nil {
		return fmt.Errorf("emit: neither EventStore nor Projector is configured — state would not be persisted")
	}
	// Enrich events with correlation metadata and global SeqNr
	for i := range events {
		events[i].CorrelationID = e.checkID
		if e.prevID != "" {
			events[i].CausationID = e.prevID
		}
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
	// Update causation chain after successful store
	if len(events) > 0 {
		e.prevID = events[len(events)-1].ID
	}
	if e.projector != nil {
		for _, ev := range events {
			if err := e.projector.Apply(ctx, ev); err != nil {
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

func (e *checkEventEmitter) EmitInboxConsumed(data domain.InboxConsumedData, now time.Time) error {
	ev, err := e.agg.RecordInboxConsumed(data, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *checkEventEmitter) EmitForceFullNextSet(prevDiv, currDiv float64, now time.Time) error {
	ev, err := e.agg.RecordForceFullNextSet(prevDiv, currDiv, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *checkEventEmitter) EmitDMailGenerated(dmail domain.DMail, now time.Time) error {
	ev, err := e.agg.RecordDMailGenerated(dmail, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *checkEventEmitter) EmitConvergenceDetected(alert domain.ConvergenceAlert, now time.Time) error {
	ev, err := e.agg.RecordConvergenceDetected(alert, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *checkEventEmitter) EmitDMailCommented(dmailName, issueID string, now time.Time) error {
	ev, err := e.agg.RecordDMailCommented(dmailName, issueID, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *checkEventEmitter) EmitCheck(result domain.CheckResult, now time.Time) error {
	events, err := e.agg.RecordCheck(result, now)
	if err != nil {
		return err
	}
	return e.emit(events...)
}

func (e *checkEventEmitter) EmitRunStarted(data domain.RunStartedData, now time.Time) error {
	ev, err := e.agg.RecordRunStarted(data, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *checkEventEmitter) EmitRunStopped(data domain.RunStoppedData, now time.Time) error {
	ev, err := e.agg.RecordRunStopped(data, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *checkEventEmitter) EmitPRConvergenceChecked(data domain.PRConvergenceCheckedData, now time.Time) error {
	ev, err := e.agg.RecordPRConvergenceChecked(data, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *checkEventEmitter) EmitPRMerged(data domain.PRMergedData, now time.Time) error {
	ev, err := e.agg.RecordPRMerged(data, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *checkEventEmitter) EmitPRMergeSkipped(data domain.PRMergeSkippedData, now time.Time) error {
	ev, err := e.agg.RecordPRMergeSkipped(data, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
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
