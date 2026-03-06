package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// Status collects current operational status from the event store and filesystem.
// gateDir is the .gate/ directory path (e.g. "<repo>/.gate").
func Status(gateDir string, logger domain.Logger) domain.StatusReport {
	var report domain.StatusReport

	// Count inbox files
	report.InboxCount = countDirFiles(gateDir, "inbox")

	// Count archive files
	report.ArchiveCount = countDirFiles(gateDir, "archive")

	// Load all events for check stats
	store := NewEventStore(gateDir, logger)
	allEvents, err := store.LoadAll()
	if err != nil || len(allEvents) == 0 {
		return report
	}

	// Count check events and compute success rate
	report.SuccessRate = domain.SuccessRate(allEvents)

	var checkCount int
	var lastCheck time.Time
	var lastDivergence float64
	var convergences int
	for _, ev := range allEvents {
		switch ev.Type {
		case domain.EventCheckCompleted:
			checkCount++
			var data domain.CheckCompletedData
			if err := json.Unmarshal(ev.Data, &data); err == nil {
				if data.Result.CheckedAt.After(lastCheck) { // nosemgrep: lod-excessive-dot-chain
					lastCheck = data.Result.CheckedAt
					lastDivergence = data.Result.Divergence
				}
			}
		case domain.EventConvergenceDetected:
			convergences++
		}
	}

	report.CheckCount = checkCount
	report.LastCheck = lastCheck
	report.Divergence = lastDivergence
	report.Convergences = convergences

	return report
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
