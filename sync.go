package amadeus

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// SyncState tracks which D-Mail × Issue pairs have been posted as comments.
type SyncState struct {
	CommentedDMails map[string]CommentRecord `json:"commented_dmails"`
}

// CommentRecord records a single D-Mail → Issue comment posting event.
type CommentRecord struct {
	DMail       string    `json:"dmail"`
	IssueID     string    `json:"issue_id"`
	CommentedAt time.Time `json:"commented_at"`
}

// LoadSyncState reads the sync state from .run/sync.json.
// Returns an empty SyncState if the file does not exist.
func (s *ProjectionStore) LoadSyncState() (SyncState, error) {
	path := filepath.Join(s.Root, ".run", "sync.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return SyncState{CommentedDMails: make(map[string]CommentRecord)}, nil
		}
		return SyncState{}, err
	}
	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return SyncState{}, err
	}
	if state.CommentedDMails == nil {
		state.CommentedDMails = make(map[string]CommentRecord)
	}
	return state, nil
}

// SaveSyncState writes the sync state to .run/sync.json.
func (s *ProjectionStore) SaveSyncState(state SyncState) error {
	path := filepath.Join(s.Root, ".run", "sync.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return s.writeJSON(path, state)
}

// PendingComment represents a D-Mail × Issue pair not yet posted as a comment.
type PendingComment struct {
	DMail       string `json:"dmail"`
	IssueID     string `json:"issue_id"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

// SyncOutput is the JSON output of `amadeus sync`.
type SyncOutput struct {
	PendingComments []PendingComment `json:"pending_comments"`
}

// MarkCommented records that a D-Mail has been posted as a comment to an issue.
// The key is "dmailName:issueID" to support multiple issues per D-Mail.
// NOTE(MY-346): Key format changed from "dmailName" to "dmailName:issueID" without migration.
// Existing sync.json with old keys will cause those D-Mails to reappear as pending.
// This is acceptable because amadeus is pre-release and no production .gate/ state exists.
func (s *ProjectionStore) MarkCommented(dmailName, issueID string) error {
	state, err := s.LoadSyncState()
	if err != nil {
		return fmt.Errorf("load sync state: %w", err)
	}
	key := dmailName + ":" + issueID
	state.CommentedDMails[key] = CommentRecord{
		DMail:       dmailName,
		IssueID:     issueID,
		CommentedAt: time.Now().UTC(),
	}
	return s.SaveSyncState(state)
}
