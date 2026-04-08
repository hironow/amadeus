package eventsource

import (
	"context"
	"fmt"
	"sync"

	"github.com/hironow/amadeus/internal/domain"
)

type eventStore interface {
	Append(ctx context.Context, events ...domain.Event) (domain.AppendResult, error)
	LoadAll(ctx context.Context) ([]domain.Event, domain.LoadResult, error)
}

// SessionRecorder wraps an event store and automatically sets CorrelationID
// and CausationID on each recorded event, with optional global SeqNr allocation.
// Thread-safe.
type SessionRecorder struct {
	store      eventStore
	seqCounter *SeqCounter // nil = no SeqNr assignment (pre-cutover)
	sessionID  string
	prevID     string
	mu         sync.Mutex
}

// NewSessionRecorder creates a SessionRecorder that resumes causation chaining
// from the last event already in the store.
func NewSessionRecorder(ctx context.Context, store eventStore, sessionID string) (*SessionRecorder, error) {
	events, _, err := store.LoadAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("new session recorder: %w", err)
	}
	var prevID string
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].CorrelationID == sessionID {
			prevID = events[i].ID
			break
		}
	}
	return &SessionRecorder{
		store:     store,
		sessionID: sessionID,
		prevID:    prevID,
	}, nil
}

// SetSeqCounter attaches a SeqCounter for global SeqNr allocation.
// When set, Record() assigns a monotonic SeqNr to each event before persistence.
func (r *SessionRecorder) SetSeqCounter(sc *SeqCounter) {
	r.seqCounter = sc
}

// Record appends a single event to the store with CorrelationID and CausationID set.
// If a SeqCounter is attached, assigns a globally monotonic SeqNr.
func (r *SessionRecorder) Record(ctx context.Context, ev domain.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ev.SessionID = r.sessionID
	ev.CorrelationID = r.sessionID
	if r.prevID != "" {
		ev.CausationID = r.prevID
	}
	if r.seqCounter != nil {
		seq, err := r.seqCounter.AllocSeqNr(ctx)
		if err != nil {
			return fmt.Errorf("alloc seq nr: %w", err)
		}
		ev.SeqNr = seq
	}
	if _, err := r.store.Append(ctx, ev); err != nil {
		return err
	}
	r.prevID = ev.ID
	return nil
}
