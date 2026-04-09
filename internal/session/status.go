package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// Status collects current operational status from the event store and filesystem.
// gateDir is the .gate/ directory path (e.g. "<repo>/.gate").
func Status(ctx context.Context, gateDir string, logger domain.Logger) domain.StatusReport {
	var report domain.StatusReport
	applyLatestProviderMetadata(ctx, gateDir, &report)

	// Count inbox files
	report.InboxCount = countDirFiles(gateDir, "inbox")

	// Count archive files
	report.ArchiveCount = countDirFiles(gateDir, "archive")

	// Load all events for check stats
	store := NewEventStore(gateDir, logger)

	allEvents, loadResult, err := store.LoadAll(ctx)
	if err != nil || len(allEvents) == 0 {
		return report
	}
	if loadResult.CorruptLineCount > 0 {
		logger.Warn("event store: %d corrupt line(s) skipped", loadResult.CorruptLineCount)
	}

	// Count check events and compute success rate
	report.SuccessRate = domain.SuccessRate(allEvents)

	var checkCount int
	var lastCheck time.Time
	var lastDivergence float64
	var convergences int
	var checkResults []domain.CheckResult
	var baselineHistory []domain.BaselinePoint
	for _, ev := range allEvents {
		switch ev.Type {
		case domain.EventCheckCompleted:
			checkCount++
			var data domain.CheckCompletedData
			if err := json.Unmarshal(ev.Data, &data); err == nil {
				checkResults = append(checkResults, data.Result)
				if data.Result.CheckedAt.After(lastCheck) { // nosemgrep: lod-excessive-dot-chain [permanent]
					lastCheck = data.Result.CheckedAt
					lastDivergence = data.Result.Divergence
				}
			}
		case domain.EventBaselineUpdated:
			var data domain.BaselineUpdatedData
			if err := json.Unmarshal(ev.Data, &data); err == nil {
				baselineHistory = append(baselineHistory, domain.BaselinePoint{
					Commit:     data.Commit,
					Divergence: data.Divergence,
					At:         ev.Timestamp,
				})
			}
		case domain.EventConvergenceDetected:
			convergences++
		}
	}

	report.CheckCount = checkCount
	report.LastCheck = lastCheck
	report.Divergence = lastDivergence
	report.Convergences = convergences
	report.BaselineHistory = baselineHistory
	report.Trend = domain.AnalyzeDivergenceTrend(checkResults)

	return report
}

func applyLatestProviderMetadata(ctx context.Context, gateDir string, report *domain.StatusReport) {
	dbPath := filepath.Join(gateDir, ".run", "sessions.db")
	store, err := NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		return
	}
	defer store.Close()
	records, err := store.List(ctx, port.ListSessionOpts{Limit: 1})
	if err != nil || len(records) == 0 {
		return
	}
	meta := records[0].Metadata
	report.ProviderState = meta[domain.MetadataProviderState]
	report.ProviderReason = meta[domain.MetadataProviderReason]
	if budget := meta[domain.MetadataProviderRetryBudget]; budget != "" {
		if n, err := strconv.Atoi(budget); err == nil {
			report.ProviderRetryBudget = n
		}
	}
	if resumeAt := meta[domain.MetadataProviderResumeAt]; resumeAt != "" {
		if ts, err := time.Parse(time.RFC3339, resumeAt); err == nil {
			report.ProviderResumeAt = ts
		}
	}
	report.ProviderResumeWhen = meta[domain.MetadataProviderResumeWhen]
}

// countDirFiles returns the number of non-directory entries in a subdirectory of gateDir.
// Returns 0 if the directory does not exist or cannot be read.
func countDirFiles(gateDir string, sub string) int {
	dir := gateDir
	if sub != "" {
		dir = filepath.Join(gateDir, sub)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	return count
}
