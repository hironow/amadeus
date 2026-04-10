package eventsource

// white-box-reason: tests SessionRecorder internals (prevID chaining, session-scoped causation) that require access to unexported eventStore interface

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// warnCapture captures Warn calls for assertion.
type warnCapture struct {
	domain.NopLogger
	warns []string
}

func (w *warnCapture) Warn(format string, args ...any) {
	w.warns = append(w.warns, fmt.Sprintf(format, args...))
}

func TestSessionRecorder_SetsCausationChain(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := NewFileEventStore(dir, &domain.NopLogger{})
	nopLog := &domain.NopLogger{}
	rec, err := NewSessionRecorder(ctx, store, "session-1", nopLog)
	if err != nil {
		t.Fatal(err)
	}

	ev1, _ := domain.NewEvent(domain.EventRunStarted, map[string]string{}, time.Now())
	if err := rec.Record(ctx, ev1); err != nil {
		t.Fatal(err)
	}

	ev2, _ := domain.NewEvent(domain.EventRunStopped, map[string]string{}, time.Now())
	if err := rec.Record(ctx, ev2); err != nil {
		t.Fatal(err)
	}

	events, _, _ := store.LoadAll(context.Background())
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].CorrelationID != "session-1" {
		t.Errorf("event[0] CorrelationID = %q, want session-1", events[0].CorrelationID)
	}
	if events[0].CausationID != "" {
		t.Errorf("event[0] CausationID should be empty, got %q", events[0].CausationID)
	}
	if events[1].CorrelationID != "session-1" {
		t.Errorf("event[1] CorrelationID = %q, want session-1", events[1].CorrelationID)
	}
	if events[1].CausationID != events[0].ID {
		t.Errorf("event[1] CausationID = %q, want %q", events[1].CausationID, events[0].ID)
	}
}

func TestSessionRecorder_ResumesPrevID(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := NewFileEventStore(dir, &domain.NopLogger{})

	nopLog := &domain.NopLogger{}
	rec1, _ := NewSessionRecorder(ctx, store, "s1", nopLog)
	ev1, _ := domain.NewEvent(domain.EventRunStarted, map[string]string{}, time.Now())
	rec1.Record(ctx, ev1)

	// Same session ID should resume CausationID chain
	rec2, _ := NewSessionRecorder(ctx, store, "s1", nopLog)
	ev2, _ := domain.NewEvent(domain.EventRunStopped, map[string]string{}, time.Now())
	rec2.Record(ctx, ev2)

	events, _, _ := store.LoadAll(context.Background())
	if events[1].CausationID != events[0].ID {
		t.Errorf("resumed recorder should chain: CausationID = %q, want %q",
			events[1].CausationID, events[0].ID)
	}
}

func TestSessionRecorder_DifferentSession_NoCausation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := NewFileEventStore(dir, &domain.NopLogger{})

	nopLog := &domain.NopLogger{}
	rec1, _ := NewSessionRecorder(ctx, store, "s1", nopLog)
	ev1, _ := domain.NewEvent(domain.EventRunStarted, map[string]string{}, time.Now())
	rec1.Record(ctx, ev1)

	// Different session should NOT chain to s1's event
	rec2, _ := NewSessionRecorder(ctx, store, "s2", nopLog)
	ev2, _ := domain.NewEvent(domain.EventRunStopped, map[string]string{}, time.Now())
	rec2.Record(ctx, ev2)

	events, _, _ := store.LoadAll(context.Background())
	if events[1].CausationID != "" {
		t.Errorf("different session should have empty CausationID, got %q", events[1].CausationID)
	}
}

func TestSessionRecorder_WarnsOnCorruptLines(t *testing.T) {
	// given: a store directory with one valid and one corrupt JSONL line
	dir := t.TempDir()
	validLine := `{"type":"run.started","aggregate_id":"test","seq_nr":1,"timestamp":"2026-01-01T00:00:00Z"}`
	corruptLine := `{CORRUPT`
	if err := os.WriteFile(
		filepath.Join(dir, "2026-01-01.jsonl"),
		[]byte(validLine+"\n"+corruptLine+"\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	// when: create recorder with warn-capturing logger
	logger := &warnCapture{}
	store := NewFileEventStore(dir, logger)
	_, err := NewSessionRecorder(context.Background(), store, "test-session", logger)
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}

	// then: logger.Warn was called with message containing "corrupt"
	found := false
	for _, w := range logger.warns {
		if strings.Contains(strings.ToLower(w), "corrupt") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about corrupt lines, got warns: %v", logger.warns)
	}
}
