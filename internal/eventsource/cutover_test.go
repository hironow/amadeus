package eventsource_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/eventsource"
)

func TestRunCutover_FirstRun(t *testing.T) {
	// given — store with 3 pre-cutover events, no SeqNr assigned
	dir := t.TempDir()
	eventsDir := filepath.Join(dir, "events")
	store := eventsource.NewFileEventStore(eventsDir, &domain.NopLogger{})
	for i := 0; i < 3; i++ {
		ev, _ := domain.NewEvent(domain.EventCheckCompleted, nil, time.Now())
		if _, err := store.Append(ev); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	snapshotDir := filepath.Join(dir, "snapshots")
	snapshotStore := eventsource.NewFileSnapshotStore(snapshotDir)
	seqCounter, err := eventsource.NewSeqCounter(filepath.Join(dir, ".run", "seq.db"))
	if err != nil {
		t.Fatalf("new seq counter: %v", err)
	}
	defer seqCounter.Close()
	ctx := context.Background()

	// when
	result, err := eventsource.RunCutover(ctx, store, snapshotStore, seqCounter, "amadeus.state", &domain.NopLogger{})
	if err != nil {
		t.Fatalf("cutover: %v", err)
	}

	// then
	if result.AlreadyDone {
		t.Error("expected AlreadyDone=false for first run")
	}
	if result.EventCount != 3 {
		t.Errorf("expected 3 pre-cutover events, got %d", result.EventCount)
	}
	if result.CutoverSeqNr != 1 {
		t.Errorf("expected CutoverSeqNr=1, got %d", result.CutoverSeqNr)
	}

	// Verify snapshot was saved
	seqNr, state, err := snapshotStore.Load(ctx, "amadeus.state")
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if seqNr != 0 {
		t.Errorf("expected snapshot SeqNr=0, got %d", seqNr)
	}
	if string(state) != "null" {
		t.Errorf("expected sentinel state 'null', got %s", state)
	}

	// Verify cutover event was appended with SeqNr=1
	events, _, _ := store.LoadAll()
	lastEvent := events[len(events)-1]
	if lastEvent.Type != domain.EventSystemCutover {
		t.Errorf("expected last event type system.cutover, got %s", lastEvent.Type)
	}
	if lastEvent.SeqNr != 1 {
		t.Errorf("expected cutover event SeqNr=1, got %d", lastEvent.SeqNr)
	}

	// Verify SeqCounter state
	latest, _ := seqCounter.LatestSeqNr(ctx)
	if latest != 1 {
		t.Errorf("expected latest SeqNr=1, got %d", latest)
	}
}

func TestRunCutover_Idempotent(t *testing.T) {
	// given — already cutover
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(filepath.Join(dir, "events"), &domain.NopLogger{})
	snapshotStore := eventsource.NewFileSnapshotStore(filepath.Join(dir, "snapshots"))
	seqCounter, _ := eventsource.NewSeqCounter(filepath.Join(dir, ".run", "seq.db"))
	defer seqCounter.Close()
	ctx := context.Background()

	// Run cutover first time
	if _, err := eventsource.RunCutover(ctx, store, snapshotStore, seqCounter, "test.state", &domain.NopLogger{}); err != nil {
		t.Fatalf("first cutover: %v", err)
	}

	// when — run again
	result, err := eventsource.RunCutover(ctx, store, snapshotStore, seqCounter, "test.state", &domain.NopLogger{})
	if err != nil {
		t.Fatalf("second cutover: %v", err)
	}

	// then — should be a no-op
	if !result.AlreadyDone {
		t.Error("expected AlreadyDone=true for second run")
	}
}

func TestRunCutover_EmptyStore(t *testing.T) {
	// given — empty event store
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(filepath.Join(dir, "events"), &domain.NopLogger{})
	snapshotStore := eventsource.NewFileSnapshotStore(filepath.Join(dir, "snapshots"))
	seqCounter, _ := eventsource.NewSeqCounter(filepath.Join(dir, ".run", "seq.db"))
	defer seqCounter.Close()

	// when
	result, err := eventsource.RunCutover(context.Background(), store, snapshotStore, seqCounter, "empty.state", &domain.NopLogger{})
	if err != nil {
		t.Fatalf("cutover: %v", err)
	}

	// then — empty store is treated as fresh install, no cutover event emitted
	if !result.AlreadyDone {
		t.Error("expected AlreadyDone=true for empty store (fresh install)")
	}
	if result.EventCount != 0 {
		t.Errorf("expected 0 events, got %d", result.EventCount)
	}
	if result.CutoverSeqNr != 0 {
		t.Errorf("expected CutoverSeqNr=0 (no cutover event), got %d", result.CutoverSeqNr)
	}
}
