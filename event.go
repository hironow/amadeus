package amadeus

import (
	"encoding/json"
	"time"
)

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
