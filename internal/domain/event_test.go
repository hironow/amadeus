package domain_test

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

func TestEventMarshalRoundTrip(t *testing.T) {
	// given
	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
	rawData := json.RawMessage(`{"commit":"abc123","check_type":"diff"}`)
	event := domain.Event{
		ID:        "test-id-001",
		Type:      domain.EventCheckCompleted,
		Timestamp: now,
		Data:      rawData,
	}

	// when
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got domain.Event
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
	types := []domain.EventType{
		domain.EventCheckCompleted,
		domain.EventBaselineUpdated,
		domain.EventForceFullNextSet,
		domain.EventDMailGenerated,
		domain.EventInboxConsumed,
		domain.EventDMailCommented,
		domain.EventConvergenceDetected,
		domain.EventArchivePruned,
		domain.EventRunStarted,
		domain.EventRunStopped,
		domain.EventPRConvergenceChecked,
	}

	seen := make(map[domain.EventType]bool)
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
	data := domain.CheckCompletedData{
		Result: domain.CheckResult{
			CheckedAt:  time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC),
			Commit:     "abc123",
			Type:       domain.CheckTypeDiff,
			Divergence: 0.42,
		},
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got domain.CheckCompletedData
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
	data := domain.DMailGeneratedData{
		DMail: domain.DMail{
			Name:        "feedback-001",
			Kind:        domain.KindDesignFeedback,
			Description: "test",
			Severity:    domain.SeverityMedium,
		},
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got domain.DMailGeneratedData
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
	data := domain.InboxConsumedData{
		Name:   "report-001",
		Kind:   domain.KindReport,
		Source: "report-001.md",
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got domain.InboxConsumedData
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
	data := domain.DMailCommentedData{
		DMail:   "feedback-001",
		IssueID: "MY-123",
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got domain.DMailCommentedData
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
	data := domain.ArchivePrunedData{
		Paths: []string{"feedback-001.md", "feedback-002.md"},
		Count: 2,
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got domain.ArchivePrunedData
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
	data := domain.ForceFullNextSetData{
		PreviousDivergence: 0.10,
		CurrentDivergence:  0.35,
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got domain.ForceFullNextSetData
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
	data := domain.BaselineUpdatedData{
		Commit:     "def456",
		Divergence: 0.25,
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got domain.BaselineUpdatedData
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
	event := domain.Event{
		ID:        "test-001",
		Type:      domain.EventCheckCompleted,
		Timestamp: time.Now(),
		Data:      json.RawMessage(`{"result":{}}`),
	}

	// when
	err := domain.ValidateEvent(event)

	// then
	if err != nil {
		t.Errorf("expected no error for valid event, got %v", err)
	}
}

func TestValidateEvent_EmptyType(t *testing.T) {
	// given
	event := domain.Event{
		ID:        "test-001",
		Type:      "",
		Timestamp: time.Now(),
		Data:      json.RawMessage(`{}`),
	}

	// when
	err := domain.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for empty Type")
	}
}

func TestValidateEvent_ZeroTimestamp(t *testing.T) {
	// given
	event := domain.Event{
		ID:   "test-001",
		Type: domain.EventCheckCompleted,
		Data: json.RawMessage(`{}`),
	}

	// when
	err := domain.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for zero Timestamp")
	}
}

func TestValidateEvent_NilData(t *testing.T) {
	// given
	event := domain.Event{
		ID:        "test-001",
		Type:      domain.EventCheckCompleted,
		Timestamp: time.Now(),
		Data:      nil,
	}

	// when
	err := domain.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for nil Data")
	}
}

func TestValidateEvent_EmptyData(t *testing.T) {
	// given
	event := domain.Event{
		ID:        "test-001",
		Type:      domain.EventCheckCompleted,
		Timestamp: time.Now(),
		Data:      json.RawMessage(``),
	}

	// when
	err := domain.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for empty Data")
	}
}

func TestValidateEvent_EmptyID(t *testing.T) {
	// given
	event := domain.Event{
		ID:        "",
		Type:      domain.EventCheckCompleted,
		Timestamp: time.Now(),
		Data:      json.RawMessage(`{}`),
	}

	// when
	err := domain.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestValidateEvent_UnknownType(t *testing.T) {
	// given: event with a typo in the type
	event := domain.Event{
		ID:        "test-001",
		Type:      domain.EventType("check.complete"), // typo: missing 'd'
		Timestamp: time.Now(),
		Data:      json.RawMessage(`{"result":{}}`),
	}

	// when
	err := domain.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for unknown event type")
	}
}

func TestValidEventType_AllConstants(t *testing.T) {
	allTypes := []domain.EventType{
		domain.EventCheckCompleted,
		domain.EventBaselineUpdated,
		domain.EventForceFullNextSet,
		domain.EventDMailGenerated,
		domain.EventInboxConsumed,
		domain.EventDMailCommented,
		domain.EventConvergenceDetected,
		domain.EventArchivePruned,
		domain.EventRunStarted,
		domain.EventRunStopped,
		domain.EventPRConvergenceChecked,
	}
	for _, et := range allTypes {
		if !domain.ValidEventType(et) {
			t.Errorf("ValidEventType(%q) = false, expected true", et)
		}
	}
}

func TestValidEventType_UnknownReturnsFalse(t *testing.T) {
	if domain.ValidEventType("totally.unknown") {
		t.Error("expected false for unknown event type")
	}
}

func TestValidateEvent_MultipleErrors(t *testing.T) {
	// given: everything is invalid
	event := domain.Event{}

	// when
	err := domain.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for fully invalid event")
	}
}

func TestTrimCheckHistory_KeepsRecentChecks(t *testing.T) {
	// given: 5 check events, keep 3
	var events []domain.Event
	for i := 0; i < 5; i++ {
		e, _ := domain.NewEvent(domain.EventCheckCompleted, domain.CheckCompletedData{
			Result: domain.CheckResult{Commit: fmt.Sprintf("commit-%d", i)},
		}, time.Now().Add(time.Duration(i)*time.Minute))
		events = append(events, e)
	}

	// when
	trimmed, dropped := domain.TrimCheckHistory(events, 3)

	// then
	if dropped != 2 {
		t.Errorf("expected 2 dropped, got %d", dropped)
	}
	if len(trimmed) != 3 {
		t.Errorf("expected 3 remaining, got %d", len(trimmed))
	}
}

func TestTrimCheckHistory_PreservesNonCheckEvents(t *testing.T) {
	// given: interleaved check and non-check events
	var events []domain.Event
	for i := 0; i < 5; i++ {
		ce, _ := domain.NewEvent(domain.EventCheckCompleted, domain.CheckCompletedData{}, time.Now())
		events = append(events, ce)
	}
	dmail, _ := domain.NewEvent(domain.EventDMailGenerated, domain.DMailGeneratedData{}, time.Now())
	events = append(events, dmail)

	// when: keep only 2 check events
	trimmed, dropped := domain.TrimCheckHistory(events, 2)

	// then: 3 check events dropped, dmail preserved
	if dropped != 3 {
		t.Errorf("expected 3 dropped, got %d", dropped)
	}
	if len(trimmed) != 3 { // 2 checks + 1 dmail
		t.Errorf("expected 3 remaining, got %d", len(trimmed))
	}
	// dmail should still be present
	hasDmail := false
	for _, e := range trimmed {
		if e.Type == domain.EventDMailGenerated {
			hasDmail = true
		}
	}
	if !hasDmail {
		t.Error("expected dmail event to be preserved")
	}
}

func TestTrimCheckHistory_BelowLimit_NoOp(t *testing.T) {
	var events []domain.Event
	for i := 0; i < 3; i++ {
		e, _ := domain.NewEvent(domain.EventCheckCompleted, domain.CheckCompletedData{}, time.Now())
		events = append(events, e)
	}

	trimmed, dropped := domain.TrimCheckHistory(events, 5)
	if dropped != 0 {
		t.Errorf("expected 0 dropped, got %d", dropped)
	}
	if len(trimmed) != 3 {
		t.Errorf("expected 3 remaining, got %d", len(trimmed))
	}
}

func TestTrimCheckHistory_DefaultMaxKeep(t *testing.T) {
	// given: maxKeep=0 should default to DefaultMaxResultHistory (100)
	var events []domain.Event
	for i := 0; i < 5; i++ {
		e, _ := domain.NewEvent(domain.EventCheckCompleted, domain.CheckCompletedData{}, time.Now())
		events = append(events, e)
	}

	trimmed, dropped := domain.TrimCheckHistory(events, 0)
	if dropped != 0 {
		t.Errorf("expected 0 dropped (5 < 100), got %d", dropped)
	}
	if len(trimmed) != 5 {
		t.Errorf("expected 5 remaining, got %d", len(trimmed))
	}
}

func TestClassifyStopReason_TableDriven(t *testing.T) {
	cases := []struct {
		reason   string
		expected domain.StopCategory
	}{
		// Graceful patterns
		{"", domain.StopGraceful},
		{"normal exit", domain.StopGraceful},
		{"context canceled", domain.StopGraceful},
		{"context deadline exceeded", domain.StopGraceful}, // must NOT match transient
		// User patterns
		{"signal", domain.StopUser},
		{"SIGTERM received", domain.StopUser},
		{"user requested shutdown", domain.StopUser},
		// IO error patterns
		{"read error", domain.StopIOError},
		{"write failed", domain.StopIOError},
		{"EOF encountered", domain.StopIOError},
		// Transient patterns
		{"timeout", domain.StopTransient},
		{"connection refused", domain.StopTransient},
		{"temporary failure", domain.StopTransient},
		// Unknown fallback
		{"unexpected panic xyz", domain.StopUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			// when
			got := domain.ClassifyStopReason(tc.reason)

			// then
			if got != tc.expected {
				t.Errorf("ClassifyStopReason(%q) = %q, want %q", tc.reason, got, tc.expected)
			}
		})
	}
}

func TestIsCriticalStop_OnlyIOErrorIsCritical(t *testing.T) {
	cases := []struct {
		cat      domain.StopCategory
		critical bool
	}{
		{domain.StopGraceful, false},
		{domain.StopUser, false},
		{domain.StopIOError, true},
		{domain.StopTransient, false},
		{domain.StopUnknown, false},
	}

	for _, tc := range cases {
		t.Run(string(tc.cat), func(t *testing.T) {
			// when
			got := domain.IsCriticalStop(tc.cat)

			// then
			if got != tc.critical {
				t.Errorf("IsCriticalStop(%q) = %v, want %v", tc.cat, got, tc.critical)
			}
		})
	}
}

func TestStopCategoryConstants(t *testing.T) {
	// given: all stop category constants must be distinct non-empty strings
	cats := []domain.StopCategory{
		domain.StopGraceful,
		domain.StopUser,
		domain.StopIOError,
		domain.StopTransient,
		domain.StopUnknown,
	}

	seen := make(map[domain.StopCategory]bool)
	for _, c := range cats {
		if c == "" {
			t.Error("found empty StopCategory constant")
		}
		if seen[c] {
			t.Errorf("duplicate StopCategory: %q", c)
		}
		seen[c] = true
	}
}

func TestConvergenceDetectedDataMarshalRoundTrip(t *testing.T) {
	// given
	data := domain.ConvergenceDetectedData{
		Alert: domain.ConvergenceAlert{
			Target:   "auth",
			Count:    5,
			Window:   14,
			DMails:   []string{"feedback-001", "feedback-002"},
			Severity: domain.SeverityHigh,
		},
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got domain.ConvergenceDetectedData
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

func TestEvent_CorrelationFields_Serialize(t *testing.T) {
	// given
	ev, err := domain.NewEvent(domain.EventRunStarted, map[string]string{"k": "v"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ev.CorrelationID = "corr-123"
	ev.CausationID = "cause-456"

	// when
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)

	// then
	if !strings.Contains(s, `"correlation_id":"corr-123"`) {
		t.Errorf("missing correlation_id: %s", s)
	}
	if !strings.Contains(s, `"causation_id":"cause-456"`) {
		t.Errorf("missing causation_id: %s", s)
	}
}

func TestEvent_CorrelationFields_OmitEmpty(t *testing.T) {
	// given
	ev, _ := domain.NewEvent(domain.EventRunStarted, map[string]string{"k": "v"}, time.Now())

	// when
	data, _ := json.Marshal(ev)

	// then
	if strings.Contains(string(data), "correlation_id") {
		t.Errorf("empty CorrelationID should be omitted")
	}
}

func TestEvent_SchemaVersion_SetByNewEvent(t *testing.T) {
	// given / when
	ev, _ := domain.NewEvent(domain.EventRunStarted, map[string]string{"k": "v"}, time.Now())

	// then
	if ev.SchemaVersion != domain.CurrentEventSchemaVersion {
		t.Errorf("got %d, want %d", ev.SchemaVersion, domain.CurrentEventSchemaVersion)
	}
}

func TestEvent_SchemaVersion_ZeroIsLegacy(t *testing.T) {
	// given
	raw := `{"id":"abc","type":"run.started","timestamp":"2026-01-01T00:00:00Z","data":{}}`

	// when
	var ev domain.Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatal(err)
	}

	// then
	if ev.SchemaVersion != 0 {
		t.Errorf("legacy event should have SchemaVersion 0, got %d", ev.SchemaVersion)
	}
}

func TestValidateEvent_RejectsFutureSchema(t *testing.T) {
	// given
	ev, _ := domain.NewEvent(domain.EventRunStarted, map[string]string{"k": "v"}, time.Now())
	ev.SchemaVersion = domain.CurrentEventSchemaVersion + 1

	// when
	err := domain.ValidateEvent(ev)

	// then
	if err == nil {
		t.Error("expected error for future schema version")
	}
	if err != nil && !strings.Contains(err.Error(), "schema_version") {
		t.Errorf("error should mention schema_version, got: %v", err)
	}
}

func TestAllEventTypes_NoDotCaseViolation(t *testing.T) {
	// Contract: every EventType constant MUST be pure dot.case (no underscores).
	dotCaseRe := regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)+$`)
	for et := range domain.AllValidEventTypes() {
		if !dotCaseRe.MatchString(string(et)) {
			t.Errorf("EventType %q violates dot.case naming convention", et)
		}
	}
}
