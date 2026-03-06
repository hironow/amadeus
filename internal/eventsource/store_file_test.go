package eventsource_test

import (
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
	result, err := store.Append(ev)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	events, loadResult, err := store.LoadAll()
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
	if _, err := store.Append(old, recent); err != nil {
		t.Fatalf("append: %v", err)
	}

	// when
	events, loadResult, err := store.LoadSince(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))
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
	_, err := store.Append(invalid)

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
	events, _, err := store.LoadAll()

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
	events, _, err := store.LoadAll()

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
