package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// PruneResult holds the outcome of an archive prune operation.
type PruneResult struct {
	ArchiveCandidates []port.PruneCandidate
	EventCandidates   []string
}

// CollectPruneCandidates finds files eligible for pruning.
// The ArchivePruneCommand is already valid by construction (parse-don't-validate).
func CollectPruneCandidates(ctx context.Context, cmd domain.ArchivePruneCommand, archiveOps port.ArchiveOps) (*PruneResult, error) {
	divRoot := filepath.Join(cmd.RepoPath().String(), domain.StateDir)
	archiveDir := filepath.Join(divRoot, "archive")
	maxAge := time.Duration(cmd.Days().Int()) * 24 * time.Hour

	archiveCandidates, err := archiveOps.FindPruneCandidates(archiveDir, maxAge)
	if err != nil {
		return nil, fmt.Errorf("find prune candidates: %w", err)
	}

	eventCandidates, err := archiveOps.ListExpiredEventFiles(ctx, divRoot, cmd.Days().Int())
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
func ExecutePrune(ctx context.Context, result *PruneResult, eventStore port.EventStore, archiveOps port.ArchiveOps, stateDir string, logger domain.Logger) (int, error) {
	totalCount := 0

	if len(result.ArchiveCandidates) > 0 {
		count, err := archiveOps.PruneFiles(result.ArchiveCandidates)
		if err != nil {
			return totalCount, fmt.Errorf("prune archive: %w", err)
		}
		totalCount += count
	}

	if len(result.EventCandidates) > 0 {
		deleted, err := archiveOps.PruneEventFiles(ctx, stateDir, result.EventCandidates)
		if err != nil {
			return totalCount, fmt.Errorf("prune event files: %w", err)
		}
		totalCount += len(deleted)
	}

	// Prune flushed outbox DB rows + incremental vacuum.
	if pruned, pruneErr := archiveOps.PruneFlushedOutbox(ctx, stateDir); pruneErr == nil && pruned > 0 {
		totalCount += pruned
	}

	// Emit archive.pruned event
	var paths []string
	for _, c := range result.ArchiveCandidates {
		paths = append(paths, filepath.Base(c.Path))
	}
	paths = append(paths, result.EventCandidates...)

	ev, evErr := domain.NewEvent(domain.EventArchivePruned, domain.ArchivePrunedData{
		Paths: paths,
		Count: totalCount,
	}, time.Now().UTC())
	if evErr != nil {
		return totalCount, fmt.Errorf("pruned %d file(s) but failed to create archive.pruned event: %w", totalCount, evErr)
	}
	if _, appendErr := eventStore.Append(ev); appendErr != nil {
		return totalCount, fmt.Errorf("pruned %d file(s) but failed to record archive.pruned event: %w", totalCount, appendErr)
	}

	return totalCount, nil
}
