package session

import (
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

func (*archiveOps) ListExpiredEventFiles(stateDir string, days int) ([]string, error) {
	return ListExpiredEventFiles(stateDir, days)
}

func (*archiveOps) PruneEventFiles(stateDir string, files []string) ([]string, error) {
	return PruneEventFiles(stateDir, files)
}

func (*archiveOps) PruneFlushedOutbox(root string) (int, error) {
	return PruneFlushedOutbox(root)
}
