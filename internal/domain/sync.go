package domain

import "time"

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

// PendingComment represents a D-Mail × Issue pair not yet posted as a comment.
type PendingComment struct {
	DMail       string `json:"dmail"`
	IssueID     string `json:"issue_id"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

// SyncOutput is the JSON output of `amadeus sync`.
type SyncOutput struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go — JSON output struct for sync command; PendingComments is a computed result list [permanent]
	PendingComments []PendingComment `json:"pending_comments"`
}
