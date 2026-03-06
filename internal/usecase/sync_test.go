package usecase

// white-box-reason: usecase internals: tests unexported fakeArchiveOps test double

import (
	"context"
	"time"

	"github.com/hironow/amadeus/internal/usecase/port"
)

// fakeArchiveOps implements port.ArchiveOps for tests.
type fakeArchiveOps struct{}

func (*fakeArchiveOps) FindPruneCandidates(_ string, _ time.Duration) ([]port.PruneCandidate, error) {
	return nil, nil
}
func (*fakeArchiveOps) PruneFiles(_ []port.PruneCandidate) (int, error) { return 0, nil }
func (*fakeArchiveOps) ListExpiredEventFiles(_ context.Context, _ string, _ int) ([]string, error) {
	return nil, nil
}
func (*fakeArchiveOps) PruneEventFiles(_ context.Context, _ string, _ []string) ([]string, error) {
	return nil, nil
}
func (*fakeArchiveOps) PruneFlushedOutbox(_ context.Context, _ string) (int, error) { return 0, nil }

// Validation tests for RunSyncCommand, RebuildCommand, and ArchivePruneCommand
// have been moved to domain/primitives_test.go (parse-don't-validate).
// The usecase layer no longer calls Validate() — commands are always-valid by construction.
