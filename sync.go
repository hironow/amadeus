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

// SyncState tracks which D-Mails have been posted as Linear comments.
type SyncState struct {
	CommentedDMails map[string]CommentRecord `json:"commented_dmails"`
}

// CommentRecord records a single D-Mail → Linear comment posting event.
type CommentRecord struct {
	DMail       string    `json:"dmail"`
	IssueID     string    `json:"issue_id"`
	CommentedAt time.Time `json:"commented_at"`
}

// LoadSyncState reads the sync state from .run/sync.json.
// Returns an empty SyncState if the file does not exist.
func (s *StateStore) LoadSyncState() (SyncState, error) {
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
func (s *StateStore) SaveSyncState(state SyncState) error {
	path := filepath.Join(s.Root, ".run", "sync.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return s.writeJSON(path, state)
}

// SyncDMailView is a JSON view of a D-Mail for sync output (issue not yet created).
type SyncDMailView struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Targets     []string `json:"targets,omitempty"`
	Body        string   `json:"body"`
}

// PendingComment represents a D-Mail linked to an issue but not yet commented.
type PendingComment struct {
	DMail       string `json:"dmail"`
	IssueID     string `json:"issue_id"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

// SyncOutput is the JSON output of `amadeus sync`.
type SyncOutput struct {
	Unsynced        []SyncDMailView  `json:"unsynced"`
	PendingComments []PendingComment `json:"pending_comments"`
}

// MarkCommented records that a D-Mail has been posted as a comment to a Linear issue.
func (s *StateStore) MarkCommented(dmailName, issueID string) error {
	state, err := s.LoadSyncState()
	if err != nil {
		return fmt.Errorf("load sync state: %w", err)
	}
	state.CommentedDMails[dmailName] = CommentRecord{
		DMail:       dmailName,
		IssueID:     issueID,
		CommentedAt: time.Now().UTC(),
	}
	return s.SaveSyncState(state)
}
