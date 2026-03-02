package amadeus_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hironow/amadeus"
)

func TestEventMarshalRoundTrip(t *testing.T) {
	// given
	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
	rawData := json.RawMessage(`{"commit":"abc123","check_type":"diff"}`)
	event := amadeus.Event{
		ID:        "test-id-001",
		Type:      amadeus.EventCheckCompleted,
		Timestamp: now,
		Data:      rawData,
	}

	// when
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got amadeus.Event
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
	types := []amadeus.EventType{
		amadeus.EventCheckCompleted,
		amadeus.EventBaselineUpdated,
		amadeus.EventForceFullNextSet,
		amadeus.EventDMailGenerated,
		amadeus.EventInboxConsumed,
		amadeus.EventDMailCommented,
		amadeus.EventConvergenceDetected,
		amadeus.EventArchivePruned,
	}

	seen := make(map[amadeus.EventType]bool)
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

func TestCheckCompletedDataMarshalRoundTrip(t *testing.T) {
	// given
	data := amadeus.CheckCompletedData{
		Result: amadeus.CheckResult{
			CheckedAt:  time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC),
			Commit:     "abc123",
			Type:       amadeus.CheckTypeDiff,
			Divergence: 0.42,
		},
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got amadeus.CheckCompletedData
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if got.Result.Commit != data.Result.Commit {
		t.Errorf("Commit = %q, want %q", got.Result.Commit, data.Result.Commit)
	}
	if got.Result.Divergence != data.Result.Divergence {
		t.Errorf("Divergence = %f, want %f", got.Result.Divergence, data.Result.Divergence)
	}
}

func TestDMailGeneratedDataMarshalRoundTrip(t *testing.T) {
	// given
	data := amadeus.DMailGeneratedData{
		DMail: amadeus.DMail{
			Name:        "feedback-001",
			Kind:        amadeus.KindFeedback,
			Description: "test",
			Severity:    amadeus.SeverityMedium,
		},
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got amadeus.DMailGeneratedData
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if got.DMail.Name != data.DMail.Name {
		t.Errorf("Name = %q, want %q", got.DMail.Name, data.DMail.Name)
	}
}

func TestInboxConsumedDataMarshalRoundTrip(t *testing.T) {
	// given
	data := amadeus.InboxConsumedData{
		Name:   "report-001",
		Kind:   amadeus.KindReport,
		Source: "report-001.md",
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got amadeus.InboxConsumedData
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if got.Name != data.Name {
		t.Errorf("Name = %q, want %q", got.Name, data.Name)
	}
	if got.Kind != data.Kind {
		t.Errorf("Kind = %q, want %q", got.Kind, data.Kind)
	}
}

func TestDMailCommentedDataMarshalRoundTrip(t *testing.T) {
	// given
	data := amadeus.DMailCommentedData{
		DMail:   "feedback-001",
		IssueID: "MY-123",
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got amadeus.DMailCommentedData
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if got.DMail != data.DMail {
		t.Errorf("DMail = %q, want %q", got.DMail, data.DMail)
	}
	if got.IssueID != data.IssueID {
		t.Errorf("IssueID = %q, want %q", got.IssueID, data.IssueID)
	}
}

func TestArchivePrunedDataMarshalRoundTrip(t *testing.T) {
	// given
	data := amadeus.ArchivePrunedData{
		Paths: []string{"feedback-001.md", "feedback-002.md"},
		Count: 2,
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got amadeus.ArchivePrunedData
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if got.Count != data.Count {
		t.Errorf("Count = %d, want %d", got.Count, data.Count)
	}
}

func TestForceFullNextSetDataMarshalRoundTrip(t *testing.T) {
	// given
	data := amadeus.ForceFullNextSetData{
		PreviousDivergence: 0.10,
		CurrentDivergence:  0.35,
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got amadeus.ForceFullNextSetData
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if got.PreviousDivergence != data.PreviousDivergence {
		t.Errorf("PreviousDivergence = %f, want %f", got.PreviousDivergence, data.PreviousDivergence)
	}
	if got.CurrentDivergence != data.CurrentDivergence {
		t.Errorf("CurrentDivergence = %f, want %f", got.CurrentDivergence, data.CurrentDivergence)
	}
}

func TestBaselineUpdatedDataMarshalRoundTrip(t *testing.T) {
	// given
	data := amadeus.BaselineUpdatedData{
		Commit:     "def456",
		Divergence: 0.25,
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got amadeus.BaselineUpdatedData
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if got.Commit != data.Commit {
		t.Errorf("Commit = %q, want %q", got.Commit, data.Commit)
	}
}

func TestValidateEvent_Valid(t *testing.T) {
	// given
	event := amadeus.Event{
		ID:        "test-001",
		Type:      amadeus.EventCheckCompleted,
		Timestamp: time.Now(),
		Data:      json.RawMessage(`{"result":{}}`),
	}

	// when
	err := amadeus.ValidateEvent(event)

	// then
	if err != nil {
		t.Errorf("expected no error for valid event, got %v", err)
	}
}

func TestValidateEvent_EmptyType(t *testing.T) {
	// given
	event := amadeus.Event{
		ID:        "test-001",
		Type:      "",
		Timestamp: time.Now(),
		Data:      json.RawMessage(`{}`),
	}

	// when
	err := amadeus.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for empty Type")
	}
}

func TestValidateEvent_ZeroTimestamp(t *testing.T) {
	// given
	event := amadeus.Event{
		ID:   "test-001",
		Type: amadeus.EventCheckCompleted,
		Data: json.RawMessage(`{}`),
	}

	// when
	err := amadeus.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for zero Timestamp")
	}
}

func TestValidateEvent_NilData(t *testing.T) {
	// given
	event := amadeus.Event{
		ID:        "test-001",
		Type:      amadeus.EventCheckCompleted,
		Timestamp: time.Now(),
		Data:      nil,
	}

	// when
	err := amadeus.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for nil Data")
	}
}

func TestValidateEvent_EmptyData(t *testing.T) {
	// given
	event := amadeus.Event{
		ID:        "test-001",
		Type:      amadeus.EventCheckCompleted,
		Timestamp: time.Now(),
		Data:      json.RawMessage(``),
	}

	// when
	err := amadeus.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for empty Data")
	}
}

func TestValidateEvent_EmptyID(t *testing.T) {
	// given
	event := amadeus.Event{
		ID:        "",
		Type:      amadeus.EventCheckCompleted,
		Timestamp: time.Now(),
		Data:      json.RawMessage(`{}`),
	}

	// when
	err := amadeus.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestValidateEvent_MultipleErrors(t *testing.T) {
	// given: everything is invalid
	event := amadeus.Event{}

	// when
	err := amadeus.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for fully invalid event")
	}
}

func TestConvergenceDetectedDataMarshalRoundTrip(t *testing.T) {
	// given
	data := amadeus.ConvergenceDetectedData{
		Alert: amadeus.ConvergenceAlert{
			Target:   "auth",
			Count:    5,
			Window:   14,
			DMails:   []string{"feedback-001", "feedback-002"},
			Severity: amadeus.SeverityHigh,
		},
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got amadeus.ConvergenceDetectedData
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if got.Alert.Target != data.Alert.Target {
		t.Errorf("Target = %q, want %q", got.Alert.Target, data.Alert.Target)
	}
	if got.Alert.Count != data.Alert.Count {
		t.Errorf("Count = %d, want %d", got.Alert.Count, data.Alert.Count)
	}
}
