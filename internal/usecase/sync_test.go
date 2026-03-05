package usecase

import (
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase/port"
)

func TestPrintSync_InvalidCommand(t *testing.T) {
	// given: empty RepoPath
	cmd := domain.RunSyncCommand{RepoPath: ""}
	a := &session.Amadeus{
		Config: domain.DefaultConfig(),
		Logger: platform.NewLogger(nil, false),
	}

	// when
	err := PrintSync(cmd, a)

	// then
	if err == nil {
		t.Fatal("expected error for empty RepoPath")
	}
	if got := err.Error(); got != "command validation: RepoPath is required" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestRebuild_InvalidCommand(t *testing.T) {
	// given: empty RepoPath
	cmd := domain.RebuildCommand{RepoPath: ""}

	// when
	err := Rebuild(cmd, nil, nil, platform.NewLogger(nil, false))

	// then
	if err == nil {
		t.Fatal("expected error for empty RepoPath")
	}
	if got := err.Error(); got != "command validation: RepoPath is required" {
		t.Fatalf("unexpected error: %s", got)
	}
}

// fakeArchiveOps implements port.ArchiveOps for tests.
type fakeArchiveOps struct{}

func (*fakeArchiveOps) FindPruneCandidates(_ string, _ time.Duration) ([]port.PruneCandidate, error) {
	return nil, nil
}
func (*fakeArchiveOps) PruneFiles(_ []port.PruneCandidate) (int, error) { return 0, nil }
func (*fakeArchiveOps) ListExpiredEventFiles(_ string, _ int) ([]string, error) {
	return nil, nil
}
func (*fakeArchiveOps) PruneEventFiles(_ string, _ []string) ([]string, error) { return nil, nil }
func (*fakeArchiveOps) PruneFlushedOutbox(_ string) (int, error)               { return 0, nil }

func TestCollectPruneCandidates_InvalidCommand(t *testing.T) {
	// given: missing Days
	cmd := domain.ArchivePruneCommand{RepoPath: "/tmp/test", Days: 0}

	// when
	_, err := CollectPruneCandidates(cmd, &fakeArchiveOps{})

	// then
	if err == nil {
		t.Fatal("expected error for zero Days")
	}
	if got := err.Error(); got != "command validation: Days must be positive" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestCollectPruneCandidates_InvalidRepoPath(t *testing.T) {
	// given: missing RepoPath
	cmd := domain.ArchivePruneCommand{RepoPath: "", Days: 30}

	// when
	_, err := CollectPruneCandidates(cmd, &fakeArchiveOps{})

	// then
	if err == nil {
		t.Fatal("expected error for empty RepoPath")
	}
	if got := err.Error(); got != "command validation: RepoPath is required" {
		t.Fatalf("unexpected error: %s", got)
	}
}
