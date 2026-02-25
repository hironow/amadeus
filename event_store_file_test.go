package amadeus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileEventStore_AppendAndLoadAll(t *testing.T) {
	// given
	dir := t.TempDir()
	store := &FileEventStore{Dir: dir}
	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)

	ev1, err := NewEvent(EventCheckCompleted, CheckCompletedData{
		Result: CheckResult{Commit: "aaa", Divergence: 0.1},
	}, now)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	ev2, err := NewEvent(EventDMailGenerated, DMailGeneratedData{
		DMail: DMail{Name: "feedback-001", Kind: KindFeedback},
	}, now.Add(time.Second))
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	// when
	if err := store.Append(ev1, ev2); err != nil {
		t.Fatalf("Append: %v", err)
	}

	events, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	// then
	if len(events) != 2 {
		t.Fatalf("LoadAll returned %d events, want 2", len(events))
	}
	if events[0].Type != EventCheckCompleted {
		t.Errorf("events[0].Type = %q, want %q", events[0].Type, EventCheckCompleted)
	}
	if events[1].Type != EventDMailGenerated {
		t.Errorf("events[1].Type = %q, want %q", events[1].Type, EventDMailGenerated)
	}
}

func TestFileEventStore_AppendCreatesJSONLFile(t *testing.T) {
	// given
	dir := t.TempDir()
	store := &FileEventStore{Dir: dir}
	now := time.Date(2026, 2, 25, 14, 30, 0, 0, time.UTC)

	ev, err := NewEvent(EventDMailCommented, DMailCommentedData{
		DMail: "feedback-001", IssueID: "MY-123",
	}, now)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	// when
	if err := store.Append(ev); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// then: file should be named YYYY-MM-DD.jsonl
	expectedFile := filepath.Join(dir, "2026-02-25.jsonl")
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("file is empty")
	}

	// verify it's valid JSON
	var parsed Event
	if err := json.Unmarshal(data[:len(data)-1], &parsed); err != nil { // trim trailing newline
		t.Fatalf("invalid JSON line: %v", err)
	}
	if parsed.Type != EventDMailCommented {
		t.Errorf("Type = %q, want %q", parsed.Type, EventDMailCommented)
	}
}

func TestFileEventStore_LoadSince(t *testing.T) {
	// given
	dir := t.TempDir()
	store := &FileEventStore{Dir: dir}

	t1 := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 2, 26, 10, 0, 0, 0, time.UTC)

	ev1, _ := NewEvent(EventCheckCompleted, CheckCompletedData{Result: CheckResult{Commit: "a"}}, t1)
	ev2, _ := NewEvent(EventCheckCompleted, CheckCompletedData{Result: CheckResult{Commit: "b"}}, t2)
	ev3, _ := NewEvent(EventCheckCompleted, CheckCompletedData{Result: CheckResult{Commit: "c"}}, t3)

	if err := store.Append(ev1); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(ev2); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(ev3); err != nil {
		t.Fatal(err)
	}

	// when
	events, err := store.LoadSince(t2)
	if err != nil {
		t.Fatalf("LoadSince: %v", err)
	}

	// then: only events after t2
	if len(events) != 1 {
		t.Fatalf("LoadSince returned %d events, want 1", len(events))
	}
	var payload CheckCompletedData
	if err := json.Unmarshal(events[0].Data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Result.Commit != "c" {
		t.Errorf("Commit = %q, want %q", payload.Result.Commit, "c")
	}
}

func TestFileEventStore_LoadAllEmptyDir(t *testing.T) {
	// given
	dir := t.TempDir()
	store := &FileEventStore{Dir: dir}

	// when
	events, err := store.LoadAll()

	// then
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("LoadAll returned %d events, want 0", len(events))
	}
}

func TestFileEventStore_LoadAllChronologicalOrder(t *testing.T) {
	// given
	dir := t.TempDir()
	store := &FileEventStore{Dir: dir}

	day1 := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)

	ev1, _ := NewEvent(EventCheckCompleted, CheckCompletedData{Result: CheckResult{Commit: "first"}}, day1)
	ev2, _ := NewEvent(EventCheckCompleted, CheckCompletedData{Result: CheckResult{Commit: "second"}}, day2)

	// Append in reverse order (day2 first, day1 second) to test sorting
	if err := store.Append(ev2); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(ev1); err != nil {
		t.Fatal(err)
	}

	// when
	events, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	// then: chronological order (day1 before day2)
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Timestamp.After(events[1].Timestamp) {
		t.Error("events not in chronological order")
	}
}

func TestNewEvent_AssignsIDAndTimestamp(t *testing.T) {
	// given
	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
	payload := DMailCommentedData{DMail: "feedback-001", IssueID: "MY-1"}

	// when
	ev, err := NewEvent(EventDMailCommented, payload, now)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	// then
	if ev.ID == "" {
		t.Error("ID is empty")
	}
	if ev.Type != EventDMailCommented {
		t.Errorf("Type = %q, want %q", ev.Type, EventDMailCommented)
	}
	if !ev.Timestamp.Equal(now) {
		t.Errorf("Timestamp = %v, want %v", ev.Timestamp, now)
	}
	if len(ev.Data) == 0 {
		t.Error("Data is empty")
	}
}

func TestNewEvent_UniqueIDs(t *testing.T) {
	// given
	now := time.Now().UTC()
	payload := DMailCommentedData{DMail: "fb-001", IssueID: "MY-1"}

	// when
	ev1, _ := NewEvent(EventDMailCommented, payload, now)
	ev2, _ := NewEvent(EventDMailCommented, payload, now)

	// then
	if ev1.ID == ev2.ID {
		t.Error("two events have the same ID")
	}
}
