package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	amadeus "github.com/hironow/amadeus"
)

// EventStore is the append-only event persistence interface.
type EventStore interface {
	// Append persists one or more events. Validation is performed before any writes.
	Append(events ...Event) error

	// LoadAll returns all events in chronological order.
	LoadAll() ([]Event, error)

	// LoadSince returns events with timestamps after the given time.
	LoadSince(after time.Time) ([]Event, error)
}

// EventApplier applies domain events to update materialized projections.
type EventApplier interface {
	// Apply processes a single event and updates the relevant projections.
	Apply(event Event) error

	// Rebuild replays all events to regenerate projections from scratch.
	Rebuild(events []Event) error
}

// EventDispatcher dispatches domain events to policy handlers.
// Implemented by usecase.PolicyEngine; injected into session via Amadeus struct.
type EventDispatcher interface {
	Dispatch(ctx context.Context, event Event) error
}

// EventType identifies the kind of domain event.
type EventType string

const (
	EventCheckCompleted      EventType = "check.completed"
	EventBaselineUpdated     EventType = "baseline.updated"
	EventForceFullNextSet    EventType = "force_full_next.set"
	EventDMailGenerated      EventType = "dmail.generated"
	EventInboxConsumed       EventType = "inbox.consumed"
	EventDMailCommented      EventType = "dmail.commented"
	EventConvergenceDetected EventType = "convergence.detected"
	EventArchivePruned       EventType = "archive.pruned"
)

// Event is the envelope for all domain events in the event store.
type Event struct {
	ID        string          `json:"id"`
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// ValidateEvent checks that an Event has all required fields populated.
// Returns an error describing all validation failures.
func ValidateEvent(e Event) error {
	var errs []string
	if e.ID == "" {
		errs = append(errs, "ID is required")
	}
	if e.Type == "" {
		errs = append(errs, "Type is required")
	}
	if e.Timestamp.IsZero() {
		errs = append(errs, "Timestamp must not be zero")
	}
	if len(e.Data) == 0 {
		errs = append(errs, "Data must not be empty")
	}
	if len(errs) > 0 {
		return errors.New("invalid event: " + strings.Join(errs, "; "))
	}
	return nil
}

// CheckCompletedData is the payload for EventCheckCompleted.
type CheckCompletedData struct {
	Result amadeus.CheckResult `json:"result"`
}

// BaselineUpdatedData is the payload for EventBaselineUpdated.
type BaselineUpdatedData struct {
	Commit     string  `json:"commit"`
	Divergence float64 `json:"divergence"`
}

// ForceFullNextSetData is the payload for EventForceFullNextSet.
type ForceFullNextSetData struct {
	PreviousDivergence float64 `json:"previous_divergence"`
	CurrentDivergence  float64 `json:"current_divergence"`
}

// DMailGeneratedData is the payload for EventDMailGenerated.
type DMailGeneratedData struct {
	DMail amadeus.DMail `json:"dmail"`
}

// InboxConsumedData is the payload for EventInboxConsumed.
type InboxConsumedData struct {
	Name   string            `json:"name"`
	Kind   amadeus.DMailKind `json:"kind"`
	Source string            `json:"source"`
}

// DMailCommentedData is the payload for EventDMailCommented.
type DMailCommentedData struct {
	DMail   string `json:"dmail"`
	IssueID string `json:"issue_id"`
}

// ConvergenceDetectedData is the payload for EventConvergenceDetected.
type ConvergenceDetectedData struct {
	Alert amadeus.ConvergenceAlert `json:"alert"`
}

// ArchivePrunedData is the payload for EventArchivePruned.
type ArchivePrunedData struct {
	Paths []string `json:"paths"`
	Count int      `json:"count"`
}

// NewEvent creates a new Event with a UUID, the given timestamp, and marshaled data payload.
func NewEvent(eventType EventType, data any, timestamp time.Time) (Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event data: %w", err)
	}
	return Event{
		ID:        uuid.NewString(),
		Type:      eventType,
		Timestamp: timestamp,
		Data:      raw,
	}, nil
}
