package domain

import (
	"fmt"
	"time"
)

// Review intake events (refs issue 0032, decision D2(a)): the reviewer
// write path. refresh_reviews ingests a GitHub open-PR snapshot into
// the gate event store; post_comment records the posted review so
// next_review can serve the oldest un-reviewed PR (intake contract).
const (
	// EventPRSnapshotIngested records an on-demand GitHub open-PR
	// snapshot (no divergence semantics — that judgment belongs to the
	// claude-code session, not this event).
	EventPRSnapshotIngested EventType = "pr.snapshot.ingested"
	// EventReviewPosted records that a review comment was posted for a
	// PR, removing it from the pending intake queue.
	EventReviewPosted EventType = "review.posted"
)

// PRSnapshotEntry is the minimal PR identity carried in a snapshot.
type PRSnapshotEntry struct { // nosemgrep: domain-primitives.public-string-field-go,structure.multiple-exported-structs-go -- JSON wire format (event payload); review-intake type family co-location is intentional [permanent]
	Number     string `json:"number"`
	Title      string `json:"title"`
	BaseBranch string `json:"base_branch"`
	HeadBranch string `json:"head_branch"`
	HeadSHA    string `json:"head_sha,omitempty"`
}

// PRSnapshotIngestedData is the EventPRSnapshotIngested payload.
type PRSnapshotIngestedData struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go,structure.multiple-exported-structs-go -- JSON wire format (event payload) [permanent]
	IngestedAt time.Time         `json:"ingested_at"`
	PRs        []PRSnapshotEntry `json:"prs"`
}

// ReviewPostedData is the EventReviewPosted payload.
type ReviewPostedData struct { // nosemgrep: structure.multiple-exported-structs-go -- review-intake type family co-location is intentional [permanent]
	PRNumber string    `json:"pr_number"`
	PostedAt time.Time `json:"posted_at"`
}

// NewPRSnapshotIngestedEvent builds the snapshot event. An empty PR
// list is valid (= no open PRs at ingest time).
func NewPRSnapshotIngestedEvent(prs []PRSnapshotEntry, now time.Time) (Event, error) {
	return NewEvent(EventPRSnapshotIngested, PRSnapshotIngestedData{IngestedAt: now, PRs: prs}, now)
}

// NewReviewPostedEvent builds the review-posted event. PRNumber is
// required (always-valid command data).
func NewReviewPostedEvent(prNumber string, now time.Time) (Event, error) {
	if prNumber == "" {
		return Event{}, fmt.Errorf("review posted: pr number is required")
	}
	return NewEvent(EventReviewPosted, ReviewPostedData{PRNumber: prNumber, PostedAt: now}, now)
}

// DefaultReviewBaseBranch is the base branch refresh_reviews targets
// when the session does not specify one.
const DefaultReviewBaseBranch = "main"

// NormalizeBaseBranch applies the domain default for an unspecified
// base branch (Parse-Don't-Validate: the session layer never invents
// fallbacks).
func NormalizeBaseBranch(base string) string {
	if base == "" {
		return DefaultReviewBaseBranch
	}
	return base
}

// PRStatesToSnapshotEntries maps PRState records to snapshot entries.
func PRStatesToSnapshotEntries(prs []PRState) []PRSnapshotEntry {
	entries := make([]PRSnapshotEntry, 0, len(prs))
	for _, pr := range prs {
		entries = append(entries, PRSnapshotEntry{
			Number:     pr.Number(),
			Title:      pr.Title(),
			BaseBranch: pr.BaseBranch(),
			HeadBranch: pr.HeadBranch(),
			HeadSHA:    pr.HeadSHA(),
		})
	}
	return entries
}
