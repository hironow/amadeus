package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// StatusReport holds operational status information for the amadeus tool.
type StatusReport struct {
	LastCheck    time.Time `json:"last_check"`
	Divergence   float64   `json:"divergence"`
	CheckCount   int       `json:"check_count"`
	InboxCount   int       `json:"inbox_count"`
	ArchiveCount int       `json:"archive_count"`
	SuccessRate  float64   `json:"success_rate"`
	Convergences int       `json:"convergences"`
}

// Status collects current operational status from the event store and filesystem.
// gateDir is the .gate/ directory path (e.g. "<repo>/.gate").
func Status(gateDir string) StatusReport {
	var report StatusReport

	// Count inbox files
	report.InboxCount = countDirFiles(gateDir, "inbox")

	// Count archive files
	report.ArchiveCount = countDirFiles(gateDir, "archive")

	// Load all events for check stats
	store := NewEventStore(gateDir)
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
				if data.Result.CheckedAt.After(lastCheck) {
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

// FormatText returns a human-readable status report string suitable for stderr.
func (r StatusReport) FormatText() string {
	var b strings.Builder
	b.WriteString("amadeus status:\n")

	// Last check
	if r.LastCheck.IsZero() {
		b.WriteString("  Last check:    no checks yet\n")
	} else {
		b.WriteString(fmt.Sprintf("  Last check:    %s\n", r.LastCheck.Format(time.RFC3339)))
	}

	// Divergence
	b.WriteString(fmt.Sprintf("  Divergence:    %.2f\n", r.Divergence))

	// Checks
	b.WriteString(fmt.Sprintf("  Checks:        %d total\n", r.CheckCount))

	// Success rate
	if r.CheckCount == 0 {
		b.WriteString("  Success rate:  no events\n")
	} else {
		b.WriteString(fmt.Sprintf("  Success rate:  %.1f%%\n", r.SuccessRate*100))
	}

	// Inbox
	b.WriteString(fmt.Sprintf("  Inbox:         %d pending\n", r.InboxCount))

	// Archive
	b.WriteString(fmt.Sprintf("  Archive:       %d processed\n", r.ArchiveCount))

	// Convergences
	b.WriteString(fmt.Sprintf("  Convergences:  %d active\n", r.Convergences))

	return b.String()
}

// FormatJSON returns the status report as a compact JSON string.
func (r StatusReport) FormatJSON() string {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}
