package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// realGetInsights exposes the verifier's learning loop to the session
// (refs issue 0034 P4): persisted insight-ledger files
// (.gate/insights/*.md) plus a live review summary derived from the
// gate event store (reviews posted / latest snapshot / pending /
// latest check divergence). Read-only and idempotent; empty state
// returns empty arrays, not an error.
func realGetInsights(ctx context.Context, gateDir string, logger domain.Logger, args json.RawMessage) map[string]any {
	var payload struct {
		Kind string `json:"kind"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	if gateDir == "" {
		return jsonResult(map[string]any{
			"initialized": false,
			"reason":      "amadeus mcp gateDir not configured (start `amadeus mcp` from the project root)",
		})
	}

	insightsDir := filepath.Join(gateDir, "insights")
	runDir := filepath.Join(gateDir, ".run")
	writer := NewInsightWriter(insightsDir, runDir)

	files := []map[string]any{}
	if entries, err := os.ReadDir(insightsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			if payload.Kind != "" && !strings.HasPrefix(e.Name(), payload.Kind) {
				continue
			}
			file, readErr := writer.Read(e.Name())
			if readErr != nil {
				continue
			}
			entryMaps := make([]map[string]any, 0, len(file.Entries))
			for _, ie := range file.Entries {
				entryMaps = append(entryMaps, map[string]any{
					"title":       ie.Title,
					"what":        ie.What,
					"why":         ie.Why,
					"how":         ie.How,
					"when":        ie.When,
					"who":         ie.Who,
					"constraints": ie.Constraints,
					"extra":       ie.Extra,
				})
			}
			files = append(files, map[string]any{
				"file":       e.Name(),
				"kind":       file.Kind,
				"updated_at": file.UpdatedAt,
				"entries":    entryMaps,
			})
		}
	}

	live := map[string]any{
		"reviews_posted":       0,
		"latest_snapshot_size": 0,
		"pending_reviews":      0,
	}
	store := NewEventStore(gateDir, logger)
	if events, _, err := store.LoadAll(ctx); err == nil {
		posted := 0
		var latestCheck *domain.CheckResult
		for _, ev := range events {
			switch ev.Type {
			case domain.EventReviewPosted:
				posted++
			case domain.EventCheckCompleted:
				var data domain.CheckCompletedData
				if jsonErr := json.Unmarshal(ev.Data, &data); jsonErr == nil {
					if latestCheck == nil || data.Result.CheckedAt.After(latestCheck.CheckedAt) { // nosemgrep: lod-excessive-dot-chain -- event payload accessor, same justification as realNextReview [permanent]
						r := data.Result
						latestCheck = &r
					}
				}
			}
		}
		live["reviews_posted"] = posted
		if intake := loadReviewIntake(events); intake != nil {
			live["latest_snapshot_size"] = len(intake.snapshot.PRs)
			live["pending_reviews"] = len(intake.pending)
			live["latest_snapshot_at"] = intake.snapshot.IngestedAt
		}
		if latestCheck != nil {
			live["latest_check_divergence"] = latestCheck.Divergence
			live["latest_check_at"] = latestCheck.CheckedAt.Format(time.RFC3339)
		}
	}

	return jsonResult(map[string]any{
		"initialized": true,
		"gateDir":     gateDir,
		"insights":    files,
		"live_review": live,
		"instruction": fmt.Sprintf("Review past corrections before judging: recurring failure classes in the ledger should raise scrutiny on matching axes. %d persisted file(s); %v review(s) posted to date.", len(files), live["reviews_posted"]),
	})
}
