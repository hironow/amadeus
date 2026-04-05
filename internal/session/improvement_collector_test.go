package session

// white-box-reason: session internals: tests collector normalization and SQLite cursor/dedup behavior

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

type fakeImprovementFeedbackSource struct {
	rows []ImprovementFeedbackRow
}

func (f fakeImprovementFeedbackSource) QueryFeedback(_ context.Context, query ImprovementFeedbackQuery) ([]ImprovementFeedbackRow, error) {
	var out []ImprovementFeedbackRow
	for _, row := range f.rows {
		if row.CreatedAt.Before(query.CreatedAfter) {
			continue
		}
		if row.CreatedAt.Equal(query.CreatedAfter) && query.AfterFeedback != "" && row.ID <= query.AfterFeedback {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func TestImprovementCollectorPollOnce_AppendsNormalizedEntryOnce(t *testing.T) {
	base := t.TempDir()
	insightsDir := filepath.Join(base, "insights")
	runDir := filepath.Join(base, ".run")
	if err := os.MkdirAll(insightsDir, 0o755); err != nil {
		t.Fatalf("mkdir insights: %v", err)
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run: %v", err)
	}
	writer := NewInsightWriter(insightsDir, runDir)
	store, err := NewSQLiteImprovementCollectorStore(filepath.Join(runDir, "improvement-ingestion.db"))
	if err != nil {
		t.Fatalf("NewSQLiteImprovementCollectorStore: %v", err)
	}
	defer store.Close()

	createdAt := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	collector := &ImprovementCollector{
		ProjectID: "proj-1",
		Source: fakeImprovementFeedbackSource{
			rows: []ImprovementFeedbackRow{{
				ID:           "fb-1",
				ProjectID:    "proj-1",
				WeaveRef:     "call-1",
				FeedbackType: "comment",
				CreatedAt:    createdAt,
				Payload: map[string]any{
					"failure_type":      "execution_failure",
					"severity":          "HIGH",
					"target_agent":      "paintress",
					"corrective_action": "retry",
					"correlation_id":    "corr-1",
					"trace_id":          "trace-1",
				},
			}},
		},
		Store:  store,
		Ledger: writer,
		Logger: &domain.NopLogger{},
	}

	processed, err := collector.PollOnce(context.Background(), 10)
	if err != nil {
		t.Fatalf("PollOnce: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	processed, err = collector.PollOnce(context.Background(), 10)
	if err != nil {
		t.Fatalf("second PollOnce: %v", err)
	}
	if processed != 0 {
		t.Fatalf("second processed = %d, want 0", processed)
	}

	file, err := writer.Read("improvement-loop.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(file.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(file.Entries))
	}
	entry := file.Entries[0]
	if entry.Extra["failure-type"] != "execution_failure" {
		t.Fatalf("failure-type = %q, want execution_failure", entry.Extra["failure-type"])
	}
	if entry.Extra["severity"] != "high" {
		t.Fatalf("severity = %q, want high", entry.Extra["severity"])
	}
	if entry.Extra["feedback-id"] != "fb-1" {
		t.Fatalf("feedback-id = %q, want fb-1", entry.Extra["feedback-id"])
	}
}

func TestImprovementCollectorPollOnce_RecordsIgnoredFeedback(t *testing.T) {
	base := t.TempDir()
	insightsDir := filepath.Join(base, "insights")
	runDir := filepath.Join(base, ".run")
	if err := os.MkdirAll(insightsDir, 0o755); err != nil {
		t.Fatalf("mkdir insights: %v", err)
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run: %v", err)
	}
	writer := NewInsightWriter(insightsDir, runDir)
	store, err := NewSQLiteImprovementCollectorStore(filepath.Join(runDir, "improvement-ingestion.db"))
	if err != nil {
		t.Fatalf("NewSQLiteImprovementCollectorStore: %v", err)
	}
	defer store.Close()

	collector := &ImprovementCollector{
		ProjectID: "proj-1",
		Source: fakeImprovementFeedbackSource{
			rows: []ImprovementFeedbackRow{{
				ID:           "fb-2",
				ProjectID:    "proj-1",
				WeaveRef:     "call-2",
				FeedbackType: "comment",
				CreatedAt:    time.Date(2026, 4, 5, 12, 1, 0, 0, time.UTC),
				Payload: map[string]any{
					"severity": "medium",
				},
			}},
		},
		Store:  store,
		Ledger: writer,
		Logger: &domain.NopLogger{},
	}

	processed, err := collector.PollOnce(context.Background(), 10)
	if err != nil {
		t.Fatalf("PollOnce: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	file, err := writer.Read("improvement-loop.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	entry := file.Entries[0]
	if entry.Extra["outcome"] != string(domain.ImprovementOutcomeIgnored) {
		t.Fatalf("outcome = %q, want %q", entry.Extra["outcome"], domain.ImprovementOutcomeIgnored)
	}
	if entry.Extra["ignored-reason"] != "missing-failure-type" {
		t.Fatalf("ignored-reason = %q, want missing-failure-type", entry.Extra["ignored-reason"])
	}
}

func TestNormalizeImprovementFeedback_SurfaceSignals(t *testing.T) {
	createdAt := time.Date(2026, 4, 5, 12, 2, 0, 0, time.UTC)
	tests := []struct {
		name              string
		row               ImprovementFeedbackRow
		wantSurface       string
		wantFailureType   string
		wantOutcome       string
		wantSecondaryType string
		wantExtraKey      string
		wantExtraValue    string
	}{
		{
			name: "ci outcome infers execution failure",
			row: ImprovementFeedbackRow{
				ID:           "sig-ci-1",
				ProjectID:    "proj-1",
				WeaveRef:     "call-ci-1",
				FeedbackType: "ci-outcome",
				CreatedAt:    createdAt,
				Payload: map[string]any{
					"ci_status":     "failed",
					"workflow_name": "build",
					"run_id":        "42",
				},
			},
			wantSurface:       "ci",
			wantFailureType:   "execution_failure",
			wantOutcome:       "failed_again",
			wantSecondaryType: "ci",
			wantExtraKey:      "ci-workflow",
			wantExtraValue:    "build",
		},
		{
			name: "pr outcome infers scope violation",
			row: ImprovementFeedbackRow{
				ID:           "sig-pr-1",
				ProjectID:    "proj-1",
				WeaveRef:     "call-pr-1",
				FeedbackType: "pr-outcome",
				CreatedAt:    createdAt,
				Payload: map[string]any{
					"pr_number":       "108",
					"review_decision": "CHANGES_REQUESTED",
				},
			},
			wantSurface:       "pr",
			wantFailureType:   "scope_violation",
			wantOutcome:       "failed_again",
			wantSecondaryType: "pr",
			wantExtraKey:      "pr-review-decision",
			wantExtraValue:    "CHANGES_REQUESTED",
		},
		{
			name: "scorer outcome infers scope violation",
			row: ImprovementFeedbackRow{
				ID:           "sig-score-1",
				ProjectID:    "proj-1",
				WeaveRef:     "call-score-1",
				FeedbackType: "score-outcome",
				CreatedAt:    createdAt,
				Payload: map[string]any{
					"scorer_verdict":      "diverged",
					"divergence_severity": "high",
				},
			},
			wantSurface:       "scorer",
			wantFailureType:   "scope_violation",
			wantOutcome:       "failed_again",
			wantSecondaryType: "scorer",
			wantExtraKey:      "divergence-severity",
			wantExtraValue:    "high",
		},
		{
			name: "trace outcome infers provider failure",
			row: ImprovementFeedbackRow{
				ID:           "sig-trace-1",
				ProjectID:    "proj-1",
				WeaveRef:     "call-trace-1",
				FeedbackType: "trace-outcome",
				CreatedAt:    createdAt,
				Payload: map[string]any{
					"trace_status": "failed",
					"trace_name":   "rerun-1",
					"error_type":   "provider_timeout",
				},
			},
			wantSurface:       "trace",
			wantFailureType:   "provider_failure",
			wantOutcome:       "failed_again",
			wantSecondaryType: "trace",
			wantExtraKey:      "trace-name",
			wantExtraValue:    "rerun-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := normalizeImprovementFeedback(tt.row)
			if got := entry.Extra["source-surface"]; got != tt.wantSurface {
				t.Fatalf("source-surface = %q, want %q", got, tt.wantSurface)
			}
			if got := entry.Extra["failure-type"]; got != tt.wantFailureType {
				t.Fatalf("failure-type = %q, want %q", got, tt.wantFailureType)
			}
			if got := entry.Extra["outcome"]; got != tt.wantOutcome {
				t.Fatalf("outcome = %q, want %q", got, tt.wantOutcome)
			}
			if got := entry.Extra["secondary-type"]; got != tt.wantSecondaryType {
				t.Fatalf("secondary-type = %q, want %q", got, tt.wantSecondaryType)
			}
			if got := entry.Extra[tt.wantExtraKey]; got != tt.wantExtraValue {
				t.Fatalf("%s = %q, want %q", tt.wantExtraKey, got, tt.wantExtraValue)
			}
		})
	}
}
