package session

import (
	"context"
	"time"

	"github.com/hironow/amadeus/internal/usecase/port"
)

// archiveOps implements port.ArchiveOps by delegating to session-level functions.
type archiveOps struct{}

// NewArchiveOps returns an ArchiveOps adapter backed by session functions.
func NewArchiveOps() port.ArchiveOps {
	return &archiveOps{}
}

func (*archiveOps) FindPruneCandidates(archiveDir string, maxAge time.Duration) ([]port.PruneCandidate, error) {
	return FindPruneCandidates(archiveDir, maxAge)
}

func (*archiveOps) PruneFiles(candidates []port.PruneCandidate) (int, error) {
	return PruneFiles(candidates)
}

func (*archiveOps) ListExpiredEventFiles(ctx context.Context, stateDir string, days int) ([]string, error) {
	return ListExpiredEventFiles(ctx, stateDir, days)
}

func (*archiveOps) PruneEventFiles(ctx context.Context, stateDir string, files []string) ([]string, error) {
	return PruneEventFiles(ctx, stateDir, files)
}

func (*archiveOps) PruneFlushedOutbox(ctx context.Context, root string) (int, error) {
	return PruneFlushedOutbox(ctx, root)
}
