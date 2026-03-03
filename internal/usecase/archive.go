package usecase

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

// PruneResult holds the outcome of an archive prune operation.
type PruneResult struct {
	ArchiveCandidates []session.PruneCandidate
	EventCandidates   []string
}

// CollectPruneCandidates finds files eligible for pruning.
// Validates the ArchivePruneCommand before collecting candidates.
func CollectPruneCandidates(cmd domain.ArchivePruneCommand) (*PruneResult, error) {
	if errs := cmd.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("command validation: %w", errs[0])
	}

	divRoot := filepath.Join(cmd.RepoPath, ".gate")
	archiveDir := filepath.Join(divRoot, "archive")
	maxAge := time.Duration(cmd.Days) * 24 * time.Hour

	archiveCandidates, err := session.FindPruneCandidates(archiveDir, maxAge)
	if err != nil {
		return nil, fmt.Errorf("find prune candidates: %w", err)
	}

	eventCandidates, err := session.ListExpiredEventFiles(divRoot, cmd.Days)
	if err != nil {
		return nil, fmt.Errorf("find expired event files: %w", err)
	}

	return &PruneResult{
		ArchiveCandidates: archiveCandidates,
		EventCandidates:   eventCandidates,
	}, nil
}

// ExecutePrune deletes the collected candidates, prunes flushed outbox rows,
// and emits an archive.pruned event.
func ExecutePrune(result *PruneResult, gateDir, eventsDir string) (int, error) {
	totalCount := 0

	if len(result.ArchiveCandidates) > 0 {
		count, err := session.PruneFiles(result.ArchiveCandidates)
		if err != nil {
			return totalCount, fmt.Errorf("prune archive: %w", err)
		}
		totalCount += count
	}

	if len(result.EventCandidates) > 0 {
		deleted, err := session.PruneEventFiles(gateDir, result.EventCandidates)
		if err != nil {
			return totalCount, fmt.Errorf("prune event files: %w", err)
		}
		totalCount += len(deleted)
	}

	// Prune flushed outbox DB rows + incremental vacuum.
	if pruned, pruneErr := session.PruneFlushedOutbox(gateDir); pruneErr == nil && pruned > 0 {
		totalCount += pruned
	}

	// Emit archive.pruned event
	var paths []string
	for _, c := range result.ArchiveCandidates {
		paths = append(paths, filepath.Base(c.Path))
	}
	paths = append(paths, result.EventCandidates...)

	eventStore := session.NewEventStoreFromEventsDir(eventsDir)
	ev, evErr := domain.NewEvent(domain.EventArchivePruned, domain.ArchivePrunedData{
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
