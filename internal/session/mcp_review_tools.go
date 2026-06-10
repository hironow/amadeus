package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// Reviewer write path (refs issue 0032, decision D2(a)). This file is
// separate from mcp_server.go to respect both the god-module guard and
// the canonical substrate locks.

// realRefreshReviews ingests the current GitHub open-PR list into the
// gate event store as an EventPRSnapshotIngested (on-demand, non-LLM —
// the daemon does not return). next_review then serves the oldest
// un-reviewed PR from the latest snapshot.
func realRefreshReviews(ctx context.Context, gateDir string, lister port.OpenPRLister, emitter port.ReviewIntakeEmitter, args json.RawMessage) map[string]any {
	var payload struct {
		BaseBranch string `json:"base_branch"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	if gateDir == "" {
		return jsonResult(map[string]any{
			"initialized": false,
			"ingested":    false,
			"reason":      "amadeus mcp gateDir not configured (start `amadeus mcp` from the project root)",
		})
	}
	if lister == nil || emitter == nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"ingested":    false,
			"persistence": "preview-only",
			"reason":      "PR lister / review emitter not wired (cmd composition root injects them; requires `gh`)",
		})
	}
	base := domain.NormalizeBaseBranch(payload.BaseBranch)
	prs, err := lister.ListOpenPRs(ctx, base)
	if err != nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"ingested":    false,
			"reason":      fmt.Sprintf("gh pr list failed: %v", err),
		})
	}
	entries := domain.PRStatesToSnapshotEntries(prs)
	if err := emitter.EmitPRSnapshotIngested(entries, time.Now().UTC()); err != nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"ingested":    false,
			"reason":      fmt.Sprintf("event append failed (re-run refresh_reviews to repair): %v", err),
		})
	}
	return jsonResult(map[string]any{
		"initialized": true,
		"ingested":    true,
		"base_branch": base,
		"pr_count":    len(entries),
		"persistence": "event-store",
	})
}

// reviewIntake is the next_review intake projection: the latest
// snapshot's PRs minus the ones with an EventReviewPosted.
type reviewIntake struct {
	snapshot *domain.PRSnapshotIngestedData
	pending  []domain.PRSnapshotEntry
}

// loadReviewIntake replays snapshot + review-posted events. Returns
// nil when no snapshot exists (legacy check.completed fallback).
func loadReviewIntake(events []domain.Event) *reviewIntake {
	var latest *domain.PRSnapshotIngestedData
	posted := map[string]bool{}
	for _, ev := range events {
		switch ev.Type {
		case domain.EventPRSnapshotIngested:
			var data domain.PRSnapshotIngestedData
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				continue
			}
			if latest == nil || data.IngestedAt.After(latest.IngestedAt) {
				d := data
				latest = &d
			}
		case domain.EventReviewPosted:
			var data domain.ReviewPostedData
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				continue
			}
			posted[data.PRNumber] = true
		}
	}
	if latest == nil {
		return nil
	}
	pending := make([]domain.PRSnapshotEntry, 0, len(latest.PRs))
	for _, pr := range latest.PRs {
		if !posted[pr.Number] {
			pending = append(pending, pr)
		}
	}
	return &reviewIntake{snapshot: latest, pending: pending}
}

// intakeResult renders the next_review response for the snapshot path.
// The oldest pending PR (= last in `gh pr list` newest-first order) is
// the next intake item.
func intakeResult(gateDir string, intake *reviewIntake) map[string]any {
	res := map[string]any{
		"initialized":   true,
		"gateDir":       gateDir,
		"source":        "pr-snapshot",
		"ingested_at":   intake.snapshot.IngestedAt,
		"snapshot_size": len(intake.snapshot.PRs),
		"pending_count": len(intake.pending),
	}
	if len(intake.pending) == 0 {
		res["none_pending"] = true
		res["instruction"] = "All snapshot PRs have a posted review. Run refresh_reviews to ingest a fresh snapshot."
		return res
	}
	next := intake.pending[len(intake.pending)-1]
	res["next_pr"] = map[string]any{
		"number":      next.Number,
		"title":       next.Title,
		"base_branch": next.BaseBranch,
		"head_branch": next.HeadBranch,
		"head_sha":    next.HeadSHA,
	}
	res["pending_prs"] = intake.pending
	res["instruction"] = "Review next_pr along the four divergence axes, post via post_comment (records review.posted), then call next_review again."
	return res
}
