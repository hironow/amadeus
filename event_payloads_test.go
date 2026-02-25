package amadeus

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCheckCompletedDataMarshalRoundTrip(t *testing.T) {
	// given
	data := CheckCompletedData{
		Result: CheckResult{
			CheckedAt:  time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC),
			Commit:     "abc123",
			Type:       CheckTypeDiff,
			Divergence: 0.42,
		},
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got CheckCompletedData
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
	data := DMailGeneratedData{
		DMail: DMail{
			Name:        "feedback-001",
			Kind:        KindFeedback,
			Description: "test",
			Severity:    SeverityMedium,
		},
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got DMailGeneratedData
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
	data := InboxConsumedData{
		Name:   "report-001",
		Kind:   KindReport,
		Source: "report-001.md",
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got InboxConsumedData
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
	data := DMailCommentedData{
		DMail:   "feedback-001",
		IssueID: "MY-123",
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got DMailCommentedData
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
	data := ArchivePrunedData{
		Paths: []string{"archive/feedback-001.md", "archive/feedback-002.md"},
		Count: 2,
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got ArchivePrunedData
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
	data := ForceFullNextSetData{
		PreviousDivergence: 0.10,
		CurrentDivergence:  0.35,
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got ForceFullNextSetData
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
	data := BaselineUpdatedData{
		Commit:     "def456",
		Divergence: 0.25,
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got BaselineUpdatedData
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if got.Commit != data.Commit {
		t.Errorf("Commit = %q, want %q", got.Commit, data.Commit)
	}
}

func TestConvergenceDetectedDataMarshalRoundTrip(t *testing.T) {
	// given
	data := ConvergenceDetectedData{
		Alert: ConvergenceAlert{
			Target:   "auth",
			Count:    5,
			Window:   14,
			DMails:   []string{"feedback-001", "feedback-002"},
			Severity: SeverityHigh,
		},
	}

	// when
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got ConvergenceDetectedData
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
