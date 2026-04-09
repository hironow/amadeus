package eventsource

import (
	"context"
	"fmt"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// CutoverResult describes what happened during cutover.
type CutoverResult struct {
	AlreadyDone bool   // true if cutover was already completed
	EventCount  int    // number of pre-cutover events found
	CutoverSeqNr uint64 // the SeqNr assigned to the cutover event (1 if performed)
}

// RunCutover performs a one-time migration from legacy (no global SeqNr) to
// the new snapshot-based event sourcing model. It is idempotent — running it
// on an already-cutover system is a no-op.
//
// Steps:
//  1. Check if SeqCounter already has allocations (next_seq > 1 → already done)
//  2. Load all existing events via store.LoadAll() (legacy path)
//  3. Save a sentinel snapshot at SeqNr=0 (marks "includes all pre-cutover events")
//  4. Initialize SeqCounter to start at 1
//  5. Emit a system.cutover event with SeqNr=1
func RunCutover(
	ctx context.Context,
	store *FileEventStore,
	snapshotStore *FileSnapshotStore,
	seqCounter *SeqCounter,
	aggregateType string,
	logger domain.Logger,
) (CutoverResult, error) {
	// Step 1: Check if already done
	latest, err := seqCounter.LatestSeqNr(ctx)
	if err != nil {
		return CutoverResult{}, fmt.Errorf("cutover: check seq counter: %w", err)
	}
	if latest > 0 {
		logger.Info("cutover: already completed (latest SeqNr=%d)", latest)
		return CutoverResult{AlreadyDone: true}, nil
	}

	// Step 2: Load all existing events
	events, _, err := store.LoadAll(ctx)
	if err != nil {
		return CutoverResult{}, fmt.Errorf("cutover: load all events: %w", err)
	}

	// No existing events = fresh install. Skip cutover — no migration needed.
	// SeqCounter is already initialized (INSERT OR IGNORE in NewSeqCounter).
	if len(events) == 0 {
		logger.Info("cutover: fresh install, no migration needed")
		return CutoverResult{AlreadyDone: true}, nil
	}
	logger.Info("cutover: found %d pre-cutover events", len(events))

	// Step 3: Save sentinel snapshot at SeqNr=0
	// Phase 1: snapshot body is "null" because projection Serialize/Deserialize
	// is not yet implemented. This sentinel marks "all pre-cutover events are
	// accounted for" — recovery will use LoadAll() (full replay) until Phase 2
	// adds real projection state. Phase 2 will overwrite this with actual
	// projection state after a full rebuild.
	if err := snapshotStore.Save(ctx, aggregateType, 0, []byte("null")); err != nil {
		return CutoverResult{}, fmt.Errorf("cutover: save sentinel snapshot: %w", err)
	}

	// Step 4: Initialize SeqCounter at 0 → first alloc returns 1
	// InitializeAt(0) sets next_seq=1, so AllocSeqNr returns 1.
	// This is a no-op if the default INSERT already set next_seq=1.

	// Step 5: Emit system.cutover event with SeqNr=1
	cutoverEvent, err := domain.NewEvent(domain.EventSystemCutover, map[string]any{
		"pre_cutover_event_count": len(events),
		"aggregate_type":          aggregateType,
	}, time.Now())
	if err != nil {
		return CutoverResult{}, fmt.Errorf("cutover: create event: %w", err)
	}
	seq, err := seqCounter.AllocSeqNr(ctx)
	if err != nil {
		return CutoverResult{}, fmt.Errorf("cutover: alloc seq nr: %w", err)
	}
	cutoverEvent.SeqNr = seq
	if _, err := store.Append(ctx, cutoverEvent); err != nil {
		return CutoverResult{}, fmt.Errorf("cutover: append cutover event: %w", err)
	}

	logger.Info("cutover: completed (SeqNr=%d, pre-cutover events=%d)", seq, len(events))
	return CutoverResult{
		EventCount:   len(events),
		CutoverSeqNr: seq,
	}, nil
}
