package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

// MY-346: MarkCommented uses composite key format "dmailName:issueID".
func TestMarkCommented_CompositeKeyFormat(t *testing.T) {
	// given
	root := t.TempDir()
	store := NewProjectionStore(root)

	// when
	if err := store.MarkCommented("feedback-001", "MY-42"); err != nil {
		t.Fatalf("MarkCommented: %v", err)
	}

	// then: key in sync.json is "feedback-001:MY-42"
	state, err := store.LoadSyncState()
	if err != nil {
		t.Fatalf("LoadSyncState: %v", err)
	}
	record, ok := state.CommentedDMails["feedback-001:MY-42"]
	if !ok {
		t.Fatalf("expected composite key 'feedback-001:MY-42', got keys: %v", keys(state.CommentedDMails))
	}
	if record.DMail != "feedback-001" {
		t.Errorf("expected dmail 'feedback-001', got %q", record.DMail)
	}
	if record.IssueID != "MY-42" {
		t.Errorf("expected issue_id 'MY-42', got %q", record.IssueID)
	}
}

// MY-346: Same D-Mail can be marked for multiple issues independently.
func TestMarkCommented_MultipleIssuesPerDMail(t *testing.T) {
	// given
	root := t.TempDir()
	store := NewProjectionStore(root)

	// when
	if err := store.MarkCommented("feedback-001", "MY-42"); err != nil {
		t.Fatalf("MarkCommented MY-42: %v", err)
	}
	if err := store.MarkCommented("feedback-001", "MY-303"); err != nil {
		t.Fatalf("MarkCommented MY-303: %v", err)
	}

	// then: both composite keys exist
	state, err := store.LoadSyncState()
	if err != nil {
		t.Fatalf("LoadSyncState: %v", err)
	}
	if _, ok := state.CommentedDMails["feedback-001:MY-42"]; !ok {
		t.Error("expected key 'feedback-001:MY-42'")
	}
	if _, ok := state.CommentedDMails["feedback-001:MY-303"]; !ok {
		t.Error("expected key 'feedback-001:MY-303'")
	}
	if len(state.CommentedDMails) != 2 {
		t.Errorf("expected 2 entries, got %d", len(state.CommentedDMails))
	}
}

// MY-346: Legacy sync.json with old key format ("dmailName" only) does not match
// the new composite key format. Those D-Mails will reappear as pending.
func TestMarkCommented_LegacyKeyNotMatched(t *testing.T) {
	// given: a legacy sync.json with old key format
	root := t.TempDir()
	runDir := filepath.Join(root, ".run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacyState := domain.SyncState{
		CommentedDMails: map[string]domain.CommentRecord{
			"feedback-001": {DMail: "feedback-001"},
		},
	}
	data, _ := json.MarshalIndent(legacyState, "", "  ")
	if err := os.WriteFile(filepath.Join(runDir, "sync.json"), data, 0o644); err != nil {
		t.Fatalf("write legacy sync.json: %v", err)
	}

	// when: loading and checking with new composite key
	store := NewProjectionStore(root)
	state, err := store.LoadSyncState()
	if err != nil {
		t.Fatalf("LoadSyncState: %v", err)
	}

	// then: legacy key exists but composite key does NOT match
	if _, ok := state.CommentedDMails["feedback-001"]; !ok {
		t.Error("expected legacy key 'feedback-001' to be readable")
	}
	if _, ok := state.CommentedDMails["feedback-001:MY-42"]; ok {
		t.Error("composite key 'feedback-001:MY-42' should NOT match legacy entry")
	}
}

func keys[V any](m map[string]V) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}
