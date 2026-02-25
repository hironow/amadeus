package amadeus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestProjector(t *testing.T) (*Projector, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".run"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "outbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(dir)
	return &Projector{Store: store}, dir
}

func TestProjector_ApplyUnknownEventReturnsNil(t *testing.T) {
	// given
	p, _ := newTestProjector(t)
	ev := Event{ID: "x", Type: "unknown.event", Timestamp: time.Now().UTC()}

	// when
	err := p.Apply(ev)

	// then
	if err != nil {
		t.Errorf("Apply unknown event returned error: %v", err)
	}
}

func TestProjector_ApplyCheckCompleted(t *testing.T) {
	// given
	p, dir := newTestProjector(t)
	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
	result := CheckResult{
		CheckedAt:  now,
		Commit:     "abc123",
		Type:       CheckTypeDiff,
		Divergence: 0.42,
	}
	ev, err := NewEvent(EventCheckCompleted, CheckCompletedData{Result: result}, now)
	if err != nil {
		t.Fatal(err)
	}

	// when
	if err := p.Apply(ev); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// then: latest.json should be updated
	latest, err := p.Store.LoadLatest()
	if err != nil {
		t.Fatalf("LoadLatest: %v", err)
	}
	if latest.Commit != "abc123" {
		t.Errorf("Commit = %q, want %q", latest.Commit, "abc123")
	}
	if latest.Divergence != 0.42 {
		t.Errorf("Divergence = %f, want %f", latest.Divergence, 0.42)
	}

	// then: history file should NOT be written by projector (events replace history)
	histDir := filepath.Join(dir, "history")
	if _, err := os.Stat(histDir); err == nil {
		entries, _ := os.ReadDir(histDir)
		if len(entries) > 0 {
			t.Error("projector should not write history files")
		}
	}
}

func TestProjector_ApplyBaselineUpdated(t *testing.T) {
	// given
	p, _ := newTestProjector(t)
	now := time.Now().UTC()
	ev, err := NewEvent(EventBaselineUpdated, BaselineUpdatedData{
		Commit: "def456", Divergence: 0.25,
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	// First set a latest so baseline has something to base on
	latestResult := CheckResult{Commit: "def456", Divergence: 0.25, Type: CheckTypeFull}
	if err := p.Store.SaveLatest(latestResult); err != nil {
		t.Fatal(err)
	}

	// when
	if err := p.Apply(ev); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// then: baseline.json should contain the result
	data, err := os.ReadFile(filepath.Join(p.Store.Root, ".run", "baseline.json"))
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	var baseline CheckResult
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatal(err)
	}
	if baseline.Commit != "def456" {
		t.Errorf("Commit = %q, want %q", baseline.Commit, "def456")
	}
}

func TestProjector_ApplyDMailGenerated(t *testing.T) {
	// given
	p, dir := newTestProjector(t)
	now := time.Now().UTC()
	dmail := DMail{
		Name:        "feedback-001",
		Kind:        KindFeedback,
		Description: "test dmail",
		Severity:    SeverityMedium,
		Body:        "some detail",
	}
	ev, err := NewEvent(EventDMailGenerated, DMailGeneratedData{DMail: dmail}, now)
	if err != nil {
		t.Fatal(err)
	}

	// when
	if err := p.Apply(ev); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// then: archive/feedback-001.md should exist
	archivePath := filepath.Join(dir, "archive", "feedback-001.md")
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive file not created: %v", err)
	}

	// then: outbox/feedback-001.md should exist
	outboxPath := filepath.Join(dir, "outbox", "feedback-001.md")
	if _, err := os.Stat(outboxPath); err != nil {
		t.Errorf("outbox file not created: %v", err)
	}
}

func TestProjector_ApplyDMailCommented(t *testing.T) {
	// given
	p, _ := newTestProjector(t)
	now := time.Now().UTC()
	ev, err := NewEvent(EventDMailCommented, DMailCommentedData{
		DMail: "feedback-001", IssueID: "MY-123",
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	// when
	if err := p.Apply(ev); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// then: sync.json should have the entry
	state, err := p.Store.LoadSyncState()
	if err != nil {
		t.Fatal(err)
	}
	key := "feedback-001:MY-123"
	record, ok := state.CommentedDMails[key]
	if !ok {
		t.Fatalf("CommentedDMails[%q] not found", key)
	}
	if record.DMail != "feedback-001" {
		t.Errorf("DMail = %q, want %q", record.DMail, "feedback-001")
	}
}

func TestProjector_ApplyInboxConsumed(t *testing.T) {
	// given
	p, _ := newTestProjector(t)
	now := time.Now().UTC()
	ev, err := NewEvent(EventInboxConsumed, InboxConsumedData{
		Name: "report-001", Kind: KindReport, Source: "report-001.md",
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	// when
	if err := p.Apply(ev); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// then: consumed.json should have the record
	records, err := p.Store.LoadConsumed()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].Name != "report-001" {
		t.Errorf("Name = %q, want %q", records[0].Name, "report-001")
	}
}

func TestProjector_ApplyForceFullNextSet(t *testing.T) {
	// given
	p, _ := newTestProjector(t)
	now := time.Now().UTC()

	// Set up an existing latest
	initial := CheckResult{Commit: "aaa", Divergence: 0.1, ForceFullNext: false}
	if err := p.Store.SaveLatest(initial); err != nil {
		t.Fatal(err)
	}

	ev, err := NewEvent(EventForceFullNextSet, ForceFullNextSetData{
		PreviousDivergence: 0.10, CurrentDivergence: 0.35,
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	// when
	if err := p.Apply(ev); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// then: latest.json should have ForceFullNext=true
	latest, err := p.Store.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}
	if !latest.ForceFullNext {
		t.Error("ForceFullNext = false, want true")
	}
}

func TestProjector_ApplyArchivePruned(t *testing.T) {
	// given
	p, dir := newTestProjector(t)
	now := time.Now().UTC()

	// Create archive files to prune
	archiveDir := filepath.Join(dir, "archive")
	f1 := filepath.Join(archiveDir, "feedback-001.md")
	f2 := filepath.Join(archiveDir, "feedback-002.md")
	os.WriteFile(f1, []byte("---\nname: feedback-001\n---\n"), 0o644)
	os.WriteFile(f2, []byte("---\nname: feedback-002\n---\n"), 0o644)

	ev, err := NewEvent(EventArchivePruned, ArchivePrunedData{
		Paths: []string{f1, f2}, Count: 2,
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	// when
	if err := p.Apply(ev); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// then: files should be deleted
	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Error("feedback-001.md should be deleted")
	}
	if _, err := os.Stat(f2); !os.IsNotExist(err) {
		t.Error("feedback-002.md should be deleted")
	}
}

func TestProjector_Rebuild(t *testing.T) {
	// given
	p, dir := newTestProjector(t)
	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)

	events := []Event{}

	// CheckCompleted event
	ev1, _ := NewEvent(EventCheckCompleted, CheckCompletedData{
		Result: CheckResult{
			CheckedAt:  now,
			Commit:     "abc",
			Type:       CheckTypeFull,
			Divergence: 0.30,
		},
	}, now)
	events = append(events, ev1)

	// DMailGenerated event
	ev2, _ := NewEvent(EventDMailGenerated, DMailGeneratedData{
		DMail: DMail{
			Name:        "feedback-001",
			Kind:        KindFeedback,
			Description: "rebuild test",
			Severity:    SeverityLow,
			Body:        "detail here",
		},
	}, now.Add(time.Second))
	events = append(events, ev2)

	// DMailCommented event
	ev3, _ := NewEvent(EventDMailCommented, DMailCommentedData{
		DMail: "feedback-001", IssueID: "MY-100",
	}, now.Add(2*time.Second))
	events = append(events, ev3)

	// InboxConsumed event
	ev4, _ := NewEvent(EventInboxConsumed, InboxConsumedData{
		Name: "report-001", Kind: KindReport, Source: "report-001.md",
	}, now.Add(3*time.Second))
	events = append(events, ev4)

	// when
	if err := p.Rebuild(events); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	// then: latest should be set
	latest, err := p.Store.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}
	if latest.Commit != "abc" {
		t.Errorf("Commit = %q, want %q", latest.Commit, "abc")
	}

	// then: archive should have the dmail
	archivePath := filepath.Join(dir, "archive", "feedback-001.md")
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive not rebuilt: %v", err)
	}

	// then: sync should have the comment
	state, err := p.Store.LoadSyncState()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state.CommentedDMails["feedback-001:MY-100"]; !ok {
		t.Error("sync state not rebuilt")
	}

	// then: consumed should have the record
	consumed, err := p.Store.LoadConsumed()
	if err != nil {
		t.Fatal(err)
	}
	if len(consumed) != 1 {
		t.Errorf("consumed count = %d, want 1", len(consumed))
	}
}
