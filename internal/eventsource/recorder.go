package eventsource

import (
	"fmt"
	"sync"

	"github.com/hironow/amadeus/internal/domain"
)

type eventStore interface {
	Append(events ...domain.Event) (domain.AppendResult, error)
	LoadAll() ([]domain.Event, domain.LoadResult, error)
}

// SessionRecorder wraps an event store and automatically sets CorrelationID
// and CausationID on each recorded event. CorrelationID is set to the session
// ID, and CausationID chains to the previous event's ID. Thread-safe.
type SessionRecorder struct {
	store     eventStore
	sessionID string
	prevID    string
	mu        sync.Mutex
}

// NewSessionRecorder creates a SessionRecorder that resumes causation chaining
// from the last event already in the store.
func NewSessionRecorder(store eventStore, sessionID string) (*SessionRecorder, error) {
	events, _, err := store.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("new session recorder: %w", err)
	}
	var prevID string
	if len(events) > 0 {
		prevID = events[len(events)-1].ID
	}
	return &SessionRecorder{
		store:     store,
		sessionID: sessionID,
		prevID:    prevID,
	}, nil
}

// Record appends a single event to the store with CorrelationID and CausationID set.
func (r *SessionRecorder) Record(ev domain.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ev.CorrelationID = r.sessionID
	if r.prevID != "" {
		ev.CausationID = r.prevID
	}
	if _, err := r.store.Append(ev); err != nil {
		return err
	}
	r.prevID = ev.ID
	return nil
}
