package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EventApplier applies domain events to update materialized projections.
type EventApplier interface {
	// Apply processes a single event and updates the relevant projections.
	Apply(event Event) error

	// Rebuild replays all events to regenerate projections from scratch.
	Rebuild(events []Event) error
}

// EventType identifies the kind of domain event.
type EventType string

const (
	EventCheckCompleted       EventType = "check.completed"
	EventBaselineUpdated      EventType = "baseline.updated"
	EventForceFullNextSet     EventType = "force_full_next.set"
	EventDMailGenerated       EventType = "dmail.generated"
	EventInboxConsumed        EventType = "inbox.consumed"
	EventDMailCommented       EventType = "dmail.commented"
	EventConvergenceDetected  EventType = "convergence.detected"
	EventArchivePruned        EventType = "archive.pruned"
	EventRunStarted           EventType = "run.started"
	EventRunStopped           EventType = "run.stopped"
	EventPRConvergenceChecked EventType = "pr_convergence.checked"
	EventPRMerged             EventType = "pr.merged"
	EventPRMergeSkipped       EventType = "pr.merge_skipped"
	EventSystemCutover        EventType = "system.cutover"
)

// validEventTypes is the set of recognized EventType values.
var validEventTypes = map[EventType]bool{
	EventCheckCompleted:       true,
	EventBaselineUpdated:      true,
	EventForceFullNextSet:     true,
	EventDMailGenerated:       true,
	EventInboxConsumed:        true,
	EventDMailCommented:       true,
	EventConvergenceDetected:  true,
	EventArchivePruned:        true,
	EventRunStarted:           true,
	EventRunStopped:           true,
	EventPRConvergenceChecked: true,
	EventPRMerged:             true,
	EventPRMergeSkipped:       true,
	EventSystemCutover:        true,
}

// ValidEventType returns true if the given EventType is recognized.
func ValidEventType(t EventType) bool {
	return validEventTypes[t]
}

// CurrentEventSchemaVersion is the schema version stamped on all new events.
const CurrentEventSchemaVersion uint8 = 1

// Event is the envelope for all domain events in the event store.
type Event struct {
	SchemaVersion uint8           `json:"schema_version,omitempty"`
	ID            string          `json:"id"`
	Type          EventType       `json:"type"`
	Timestamp     time.Time       `json:"timestamp"`
	Data          json.RawMessage `json:"data"`
	SessionID     string          `json:"session_id,omitempty"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	CausationID   string          `json:"causation_id,omitempty"`
	AggregateID   string          `json:"aggregate_id,omitempty"`
	AggregateType string          `json:"aggregate_type,omitempty"`
	SeqNr         uint64          `json:"seq_nr,omitempty"`
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
	} else if !ValidEventType(e.Type) {
		errs = append(errs, fmt.Sprintf("Type %q is not a recognized event type", e.Type))
	}
	if e.Timestamp.IsZero() {
		errs = append(errs, "Timestamp must not be zero")
	}
	if len(e.Data) == 0 {
		errs = append(errs, "Data must not be empty")
	}
	if e.SchemaVersion > CurrentEventSchemaVersion {
		errs = append(errs, fmt.Sprintf("schema_version %d exceeds supported version %d", e.SchemaVersion, CurrentEventSchemaVersion))
	}
	if len(errs) > 0 {
		return errors.New("invalid event: " + strings.Join(errs, "; "))
	}
	return nil
}

// AppendResult captures metrics from an event store Append operation.
type AppendResult struct {
	BytesWritten int // total bytes written to event files
}

// LoadResult captures metrics from an event store Load operation.
type LoadResult struct {
	FileCount        int // number of .jsonl files scanned
	CorruptLineCount int // number of lines skipped due to parse errors
}

// CheckCompletedData is the payload for EventCheckCompleted.
type CheckCompletedData struct {
	Result CheckResult `json:"result"`
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
	DMail DMail `json:"dmail"`
}

// InboxConsumedData is the payload for EventInboxConsumed.
type InboxConsumedData struct {
	Name   string    `json:"name"`
	Kind   DMailKind `json:"kind"`
	Source string    `json:"source"`
}

// DMailCommentedData is the payload for EventDMailCommented.
type DMailCommentedData struct {
	DMail   string `json:"dmail"`
	IssueID string `json:"issue_id"`
}

// ConvergenceDetectedData is the payload for EventConvergenceDetected.
type ConvergenceDetectedData struct {
	Alert ConvergenceAlert `json:"alert"`
}

// ArchivePrunedData is the payload for EventArchivePruned.
type ArchivePrunedData struct {
	Paths []string `json:"paths"`
	Count int      `json:"count"`
}

// RunStartedData is the payload for run.started events.
type RunStartedData struct {
	IntegrationBranch string `json:"integration_branch"`
	BaseBranch        string `json:"base_branch,omitempty"`
}

// RunStoppedReason constants for run.stopped event reasons.
const (
	RunStoppedReasonError         = "error"
	RunStoppedReasonSignal        = "signal"
	RunStoppedReasonChannelClosed = "channel_closed"
)

// RunStoppedData is the payload for run.stopped events.
type RunStoppedData struct {
	Reason string `json:"reason"`
}

// PRConvergenceCheckedData is the payload for pr_convergence.checked events.
type PRConvergenceCheckedData struct {
	IntegrationBranch string `json:"integration_branch"`
	TotalOpenPRs      int    `json:"total_open_prs"`
	Chains            int    `json:"chains"`
	ConflictPRs       int    `json:"conflict_prs"`
	DMails            int    `json:"dmails_generated"`
}

// PRMergedData is the payload for pr.merged events.
type PRMergedData struct {
	PRNumber string `json:"pr_number"`
	Title    string `json:"title"`
	Method   string `json:"method"` // "squash" or "merge"
}

// PRMergeSkippedData is the payload for pr.merge_skipped events.
type PRMergeSkippedData struct {
	PRNumber string   `json:"pr_number"`
	Title    string   `json:"title"`
	Reasons  []string `json:"reasons"`
}

// TrimCheckHistory keeps only the maxKeep most recent EventCheckCompleted events,
// preserving all other event types. Returns the trimmed event slice and the
// number of check events removed.
func TrimCheckHistory(events []Event, maxKeep int) ([]Event, int) {
	if maxKeep <= 0 {
		maxKeep = DefaultMaxResultHistory
	}

	// Count check events
	var checkIndices []int
	for i, e := range events {
		if e.Type == EventCheckCompleted {
			checkIndices = append(checkIndices, i)
		}
	}
	if len(checkIndices) <= maxKeep {
		return events, 0
	}

	// Build set of indices to drop (oldest checks beyond maxKeep)
	dropCount := len(checkIndices) - maxKeep
	dropSet := make(map[int]bool, dropCount)
	for i := 0; i < dropCount; i++ {
		dropSet[checkIndices[i]] = true
	}

	result := make([]Event, 0, len(events)-dropCount)
	for i, e := range events {
		if !dropSet[i] {
			result = append(result, e)
		}
	}
	return result, dropCount
}

// NewEvent creates a new Event with a UUID, the given timestamp, and marshaled data payload.
func NewEvent(eventType EventType, data any, timestamp time.Time) (Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event data: %w", err)
	}
	return Event{
		SchemaVersion: CurrentEventSchemaVersion,
		ID:            uuid.NewString(),
		Type:          eventType,
		Timestamp:     timestamp,
		Data:          raw,
	}, nil
}

// Policy represents an implicit reactive rule: WHEN [EVENT] THEN [COMMAND].
// See ADR S0014 for the POLICY pattern reference.
type Policy struct {
	Name    string    // unique identifier for the policy
	Trigger EventType // domain event that activates this policy
	Action  string    // description of the resulting command
}

// Policies registers all known implicit policies in amadeus.
// These document the existing reactive behaviors for future automation.
var Policies = []Policy{
	{Name: "CheckCompletedGenerateDMail", Trigger: EventCheckCompleted, Action: "GenerateDMail"},
	{Name: "ConvergenceDetectedNotify", Trigger: EventConvergenceDetected, Action: "NotifyConvergence"},
	{Name: "InboxConsumedUpdateProjection", Trigger: EventInboxConsumed, Action: "UpdateProjection"},
	{Name: "DMailGeneratedFlushOutbox", Trigger: EventDMailGenerated, Action: "FlushOutbox"},
}
