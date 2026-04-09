package eventsource_test

import (
	"context"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/eventsource"
)

func TestFileEventStore_AppendAndLoadAll(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	ev, err := domain.NewEvent(domain.EventCheckCompleted, map[string]string{"result": "pass"}, time.Now())
	if err != nil {
		t.Fatalf("new event: %v", err)
	}

	// when
	result, err := store.Append(context.Background(), ev)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	events, loadResult, err := store.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("load all: %v", err)
	}

	// then
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ID != ev.ID {
		t.Errorf("expected ID %s, got %s", ev.ID, events[0].ID)
	}
	if events[0].Type != domain.EventCheckCompleted {
		t.Errorf("expected type %s, got %s", domain.EventCheckCompleted, events[0].Type)
	}
	if result.BytesWritten <= 0 {
		t.Errorf("expected positive bytes written, got %d", result.BytesWritten)
	}
	if loadResult.FileCount != 1 {
		t.Errorf("expected 1 file, got %d", loadResult.FileCount)
	}
	if loadResult.CorruptLineCount != 0 {
		t.Errorf("expected 0 corrupt lines, got %d", loadResult.CorruptLineCount)
	}
}

func TestFileEventStore_LoadSince_FiltersOlderEvents(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	old, err := domain.NewEvent(domain.EventCheckCompleted, nil, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("new event: %v", err)
	}
	recent, err := domain.NewEvent(domain.EventBaselineUpdated, nil, time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("new event: %v", err)
	}
	if _, err := store.Append(context.Background(), old, recent); err != nil {
		t.Fatalf("append: %v", err)
	}

	// when
	events, loadResult, err := store.LoadSince(context.Background(), time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("load since: %v", err)
	}

	// then
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ID != recent.ID {
		t.Errorf("expected recent event, got %s", events[0].ID)
	}
	if loadResult.FileCount != 2 {
		t.Errorf("expected 2 files (2 dates), got %d", loadResult.FileCount)
	}
}

func TestFileEventStore_AppendRejectsInvalidEvent(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	invalid := domain.Event{} // missing ID, Type, Timestamp

	// when
	_, err := store.Append(context.Background(), invalid)

	// then
	if err == nil {
		t.Fatal("expected error for invalid event, got nil")
	}
}

func TestFileEventStore_LoadAll_EmptyDir(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})

	// when
	events, _, err := store.LoadAll(context.Background())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestFileEventStore_LoadAll_NonexistentDir(t *testing.T) {
	// given
	store := eventsource.NewFileEventStore("/nonexistent/path/events", &domain.NopLogger{})

	// when
	events, _, err := store.LoadAll(context.Background())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if events != nil {
		t.Errorf("expected nil events, got %v", events)
	}
}

func TestFileEventStore_ImplementsInterface(t *testing.T) {
	store := eventsource.NewFileEventStore(t.TempDir(), &domain.NopLogger{})
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestFileEventStore_LoadAfterSeqNr_FiltersAndSorts(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	now := time.Now()
	ev1, _ := domain.NewEvent(domain.EventCheckCompleted, nil, now)
	ev1.SeqNr = 1
	ev2, _ := domain.NewEvent(domain.EventCheckCompleted, nil, now.Add(time.Second))
	ev2.SeqNr = 2
	ev3, _ := domain.NewEvent(domain.EventCheckCompleted, nil, now.Add(2*time.Second))
	ev3.SeqNr = 3
	if _, err := store.Append(context.Background(), ev1, ev2, ev3); err != nil {
		t.Fatalf("append: %v", err)
	}

	// when
	events, _, err := store.LoadAfterSeqNr(context.Background(), 1)
	if err != nil {
		t.Fatalf("load after seq nr: %v", err)
	}

	// then
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].SeqNr != 2 {
		t.Errorf("expected SeqNr 2, got %d", events[0].SeqNr)
	}
	if events[1].SeqNr != 3 {
		t.Errorf("expected SeqNr 3, got %d", events[1].SeqNr)
	}
}

func TestFileEventStore_LoadAfterSeqNr_SkipsZeroSeqNr(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	legacy, _ := domain.NewEvent(domain.EventCheckCompleted, nil, time.Now())
	postCutover, _ := domain.NewEvent(domain.EventCheckCompleted, nil, time.Now().Add(time.Second))
	postCutover.SeqNr = 1
	if _, err := store.Append(context.Background(), legacy, postCutover); err != nil {
		t.Fatalf("append: %v", err)
	}

	// when
	events, _, err := store.LoadAfterSeqNr(context.Background(), 0)
	if err != nil {
		t.Fatalf("load after seq nr: %v", err)
	}

	// then
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].SeqNr != 1 {
		t.Errorf("expected SeqNr 1, got %d", events[0].SeqNr)
	}
}

func TestFileEventStore_LatestSeqNr(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	now := time.Now()
	ev1, _ := domain.NewEvent(domain.EventCheckCompleted, nil, now)
	ev1.SeqNr = 3
	ev2, _ := domain.NewEvent(domain.EventCheckCompleted, nil, now.Add(time.Second))
	ev2.SeqNr = 7
	if _, err := store.Append(context.Background(), ev1, ev2); err != nil {
		t.Fatalf("append: %v", err)
	}

	// when
	seqNr, err := store.LatestSeqNr(context.Background())
	if err != nil {
		t.Fatalf("latest seq nr: %v", err)
	}

	// then
	if seqNr != 7 {
		t.Errorf("expected 7, got %d", seqNr)
	}
}

func TestFileEventStore_LatestSeqNr_EmptyStore(t *testing.T) {
	// given
	store := eventsource.NewFileEventStore(t.TempDir(), &domain.NopLogger{})

	// when
	seqNr, err := store.LatestSeqNr(context.Background())
	if err != nil {
		t.Fatalf("latest seq nr: %v", err)
	}

	// then
	if seqNr != 0 {
		t.Errorf("expected 0, got %d", seqNr)
	}
}
