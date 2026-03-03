package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// LoadSyncState reads the sync state from .run/sync.json.
// Returns an empty SyncState if the file does not exist.
func (s *ProjectionStore) LoadSyncState() (domain.SyncState, error) {
	path := filepath.Join(s.Root, ".run", "sync.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return domain.SyncState{CommentedDMails: make(map[string]domain.CommentRecord)}, nil
		}
		return domain.SyncState{}, err
	}
	var state domain.SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return domain.SyncState{}, err
	}
	if state.CommentedDMails == nil {
		state.CommentedDMails = make(map[string]domain.CommentRecord)
	}
	return state, nil
}

// SaveSyncState writes the sync state to .run/sync.json.
func (s *ProjectionStore) SaveSyncState(state domain.SyncState) error {
	path := filepath.Join(s.Root, ".run", "sync.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return s.writeJSON(path, state)
}

// MarkCommented records that a D-Mail has been posted as a comment to an issue.
// The key is "dmailName:issueID" to support multiple issues per D-Mail.
// DECISION(MY-346): Key format changed from "dmailName" to "dmailName:issueID".
// Old sync.json with legacy keys will cause those D-Mails to reappear as pending.
// This is a finalized non-backward-compatible change; no migration is provided.
func (s *ProjectionStore) MarkCommented(dmailName, issueID string) error {
	state, err := s.LoadSyncState()
	if err != nil {
		return fmt.Errorf("load sync state: %w", err)
	}
	key := dmailName + ":" + issueID
	state.CommentedDMails[key] = domain.CommentRecord{
		DMail:       dmailName,
		IssueID:     issueID,
		CommentedAt: time.Now().UTC(),
	}
	return s.SaveSyncState(state)
}
