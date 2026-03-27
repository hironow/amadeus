package session

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/eventsource"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// Compile-time check that FileCrossRepoReader implements port.CrossRepoReader.
var _ port.CrossRepoReader = (*FileCrossRepoReader)(nil)

// FileCrossRepoReader reads divergence state from sibling tool state directories.
type FileCrossRepoReader struct {
	logger domain.Logger
}

// NewFileCrossRepoReader creates a new FileCrossRepoReader.
func NewFileCrossRepoReader(logger domain.Logger) *FileCrossRepoReader {
	return &FileCrossRepoReader{logger: logger}
}

// ReadToolSnapshot reads the latest divergence state for the given tool.
// stateDir is the absolute path to the tool's state directory (e.g. /repo/.gate).
// Returns Available=false if the state dir doesn't exist.
func (r *FileCrossRepoReader) ReadToolSnapshot(tool domain.ToolName, stateDir string) (domain.ToolSnapshot, error) {
	snap := domain.ToolSnapshot{
		Tool:      tool,
		Available: false,
	}

	// Check if state dir exists
	if _, err := os.Stat(stateDir); errors.Is(err, fs.ErrNotExist) {
		return snap, nil
	} else if err != nil {
		return snap, err
	}

	// State dir exists — mark as available
	snap.Available = true

	// Only amadeus has check.completed events with divergence scores.
	// For other tools, report Available=true with zero divergence.
	if tool != domain.ToolAmadeus {
		snap.Severity = domain.SeverityLow
		return snap, nil
	}

	// For amadeus, read the event store to find the latest check.completed event.
	eventsDir := eventsource.EventsDir(stateDir)
	if _, err := os.Stat(eventsDir); errors.Is(err, fs.ErrNotExist) {
		snap.Severity = domain.SeverityLow
		return snap, nil
	}

	store := eventsource.NewFileEventStore(eventsDir, r.logger)

	// Load events from the last 7 days to find the latest check.
	since := time.Now().AddDate(0, 0, -7)
	events, _, err := store.LoadSince(since)
	if err != nil {
		r.logger.Warn("failed to load events for %s: %v", tool, err)
		snap.Severity = domain.SeverityLow
		return snap, nil
	}

	// Find the latest check.completed event (scan in reverse).
	for i := len(events) - 1; i >= 0; i-- {
		ev := events[i]
		if ev.Type != domain.EventCheckCompleted {
			continue
		}
		var data domain.CheckCompletedData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			r.logger.Warn("failed to unmarshal check.completed event: %v", err)
			continue
		}
		snap.Divergence = data.Result.Divergence
		snap.Severity = deriveSeverityFromResult(data.Result)
		snap.LastCheck = data.Result.CheckedAt
		return snap, nil
	}

	// No check.completed events found — available but no divergence data
	snap.Severity = domain.SeverityLow
	return snap, nil
}

// deriveSeverityFromResult determines severity from a CheckResult's divergence value
// using default thresholds.
func deriveSeverityFromResult(result domain.CheckResult) domain.Severity {
	cfg := domain.DefaultThresholds()
	div := result.Divergence
	if div >= cfg.Thresholds.MediumMax {
		return domain.SeverityHigh
	}
	if div >= cfg.Thresholds.LowMax {
		return domain.SeverityMedium
	}
	return domain.SeverityLow
}

// ResolveToolStateDirs builds the map of tool -> absolute state dir path.
// repoRoot is the directory containing the current repository.
// siblingRoot is the parent directory containing sibling repositories.
func ResolveToolStateDirs(repoRoot string, siblingRoot string) map[domain.ToolName]string {
	dirs := make(map[domain.ToolName]string, len(domain.AllTools))
	for _, tool := range domain.AllTools {
		stateSubdir := domain.ToolStateDir(tool)
		if stateSubdir == "" {
			continue
		}
		if tool == domain.ToolAmadeus {
			dirs[tool] = filepath.Join(repoRoot, stateSubdir)
		} else {
			dirs[tool] = filepath.Join(siblingRoot, string(tool), stateSubdir)
		}
	}
	return dirs
}
