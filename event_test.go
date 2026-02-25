package amadeus

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventMarshalRoundTrip(t *testing.T) {
	// given
	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
	rawData := json.RawMessage(`{"commit":"abc123","check_type":"diff"}`)
	event := Event{
		ID:        "test-id-001",
		Type:      EventCheckCompleted,
		Timestamp: now,
		Data:      rawData,
	}

	// when
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Event
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if got.ID != event.ID {
		t.Errorf("ID = %q, want %q", got.ID, event.ID)
	}
	if got.Type != event.Type {
		t.Errorf("Type = %q, want %q", got.Type, event.Type)
	}
	if !got.Timestamp.Equal(event.Timestamp) {
		t.Errorf("Timestamp = %v, want %v", got.Timestamp, event.Timestamp)
	}
	if string(got.Data) != string(event.Data) {
		t.Errorf("Data = %s, want %s", got.Data, event.Data)
	}
}

func TestEventTypeConstants(t *testing.T) {
	// then: verify all event types are distinct non-empty strings
	types := []EventType{
		EventCheckCompleted,
		EventBaselineUpdated,
		EventForceFullNextSet,
		EventDMailGenerated,
		EventInboxConsumed,
		EventDMailCommented,
		EventConvergenceDetected,
		EventArchivePruned,
	}

	seen := make(map[EventType]bool)
	for _, et := range types {
		if et == "" {
			t.Error("found empty EventType constant")
		}
		if seen[et] {
			t.Errorf("duplicate EventType: %q", et)
		}
		seen[et] = true
	}
}
