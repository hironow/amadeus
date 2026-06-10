package port

import (
	"context"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// OpenPRLister is the narrow read surface refresh_reviews needs: list
// the open PRs for a target branch. GhPRReader satisfies it (subset of
// GitHubPRReader; pattern: dominator ADR 0005 narrow injection).
type OpenPRLister interface { // nosemgrep: structure.multiple-exported-interfaces-go -- review-intake port family co-location is intentional [permanent]
	ListOpenPRs(ctx context.Context, targetBranch string) ([]domain.PRState, error)
}

// ReviewIntakeEmitter is the narrow write surface the MCP data plane
// needs for the reviewer write path (refs issue 0032 D2(a)):
// snapshot ingestion + review-posted ledger entries.
type ReviewIntakeEmitter interface { // nosemgrep: structure.multiple-exported-interfaces-go -- review-intake port family co-location is intentional [permanent]
	EmitPRSnapshotIngested(prs []domain.PRSnapshotEntry, now time.Time) error
	EmitReviewPosted(prNumber string, now time.Time) error
}
