package amadeus

import (
	"path/filepath"
	"testing"
)

func TestLoadSyncState_Empty(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// when
	state, err := store.LoadSyncState()

	// then
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(state.CommentedDMails) != 0 {
		t.Errorf("expected empty map, got %d entries", len(state.CommentedDMails))
	}
}

func TestSyncState_RoundTrip(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// when: mark a D-Mail as commented
	if err := store.MarkCommented("feedback-001", "MY-250"); err != nil {
		t.Fatalf("MarkCommented failed: %v", err)
	}

	// then: load and verify
	state, err := store.LoadSyncState()
	if err != nil {
		t.Fatalf("LoadSyncState failed: %v", err)
	}
	record, ok := state.CommentedDMails["feedback-001"]
	if !ok {
		t.Fatal("expected feedback-001 in CommentedDMails")
	}
	if record.IssueID != "MY-250" {
		t.Errorf("expected issue_id MY-250, got %s", record.IssueID)
	}
	if record.DMail != "feedback-001" {
		t.Errorf("expected dmail feedback-001, got %s", record.DMail)
	}
	if record.CommentedAt.IsZero() {
		t.Error("expected CommentedAt to be set")
	}
}

func TestMarkCommented_Appends(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// when: mark two different D-Mails
	if err := store.MarkCommented("feedback-001", "MY-250"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkCommented("feedback-002", "MY-251"); err != nil {
		t.Fatal(err)
	}

	// then: both should exist
	state, err := store.LoadSyncState()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.CommentedDMails) != 2 {
		t.Errorf("expected 2 entries, got %d", len(state.CommentedDMails))
	}
}
