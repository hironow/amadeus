package usecase

import (
	"fmt"
	"path/filepath"
	"time"

	amadeus "github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/session"
)

// PruneResult holds the outcome of an archive prune operation.
type PruneResult struct {
	ArchiveCandidates []session.PruneCandidate
	EventCandidates   []session.ExpiredEventFile
}

// CollectPruneCandidates finds files eligible for pruning.
// Validates the ArchivePruneCommand before collecting candidates.
func CollectPruneCandidates(cmd amadeus.ArchivePruneCommand) (*PruneResult, error) {
	if errs := cmd.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("command validation: %w", errs[0])
	}

	divRoot := filepath.Join(cmd.RepoPath, ".gate")
	archiveDir := filepath.Join(divRoot, "archive")
	eventsDir := filepath.Join(divRoot, "events")
	maxAge := time.Duration(cmd.Days) * 24 * time.Hour

	archiveCandidates, err := session.FindPruneCandidates(archiveDir, maxAge)
	if err != nil {
		return nil, fmt.Errorf("find prune candidates: %w", err)
	}

	eventCandidates, err := session.FindExpiredEventFiles(eventsDir, maxAge)
	if err != nil {
		return nil, fmt.Errorf("find expired event files: %w", err)
	}

	return &PruneResult{
		ArchiveCandidates: archiveCandidates,
		EventCandidates:   eventCandidates,
	}, nil
}

// ExecutePrune deletes the collected candidates and emits an archive.pruned event.
func ExecutePrune(result *PruneResult, eventsDir string) (int, error) {
	totalCount := 0

	if len(result.ArchiveCandidates) > 0 {
		count, err := session.PruneFiles(result.ArchiveCandidates)
		if err != nil {
			return totalCount, fmt.Errorf("prune archive: %w", err)
		}
		totalCount += count
	}

	if len(result.EventCandidates) > 0 {
		count, err := session.PruneEventFiles(result.EventCandidates)
		if err != nil {
			return totalCount, fmt.Errorf("prune event files: %w", err)
		}
		totalCount += count
	}

	// Emit archive.pruned event
	var paths []string
	for _, c := range result.ArchiveCandidates {
		paths = append(paths, filepath.Base(c.Path))
	}
	for _, c := range result.EventCandidates {
		paths = append(paths, filepath.Base(c.Path))
	}

	eventStore := session.NewEventStoreFromEventsDir(eventsDir)
	ev, evErr := amadeus.NewEvent(amadeus.EventArchivePruned, amadeus.ArchivePrunedData{
		Paths: paths,
		Count: totalCount,
	}, time.Now().UTC())
	if evErr != nil {
		return totalCount, fmt.Errorf("pruned %d file(s) but failed to create archive.pruned event: %w", totalCount, evErr)
	}
	if appendErr := eventStore.Append(ev); appendErr != nil {
		return totalCount, fmt.Errorf("pruned %d file(s) but failed to record archive.pruned event: %w", totalCount, appendErr)
	}

	return totalCount, nil
}
