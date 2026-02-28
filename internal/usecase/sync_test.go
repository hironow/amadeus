package usecase

import (
	"testing"

	amadeus "github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/session"
)

func TestPrintSync_InvalidCommand(t *testing.T) {
	// given: empty RepoPath
	cmd := amadeus.RunSyncCommand{RepoPath: ""}
	a := &session.Amadeus{
		Config: amadeus.DefaultConfig(),
		Logger: amadeus.NewLogger(nil, false),
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
	cmd := amadeus.RebuildCommand{RepoPath: ""}

	// when
	err := Rebuild(cmd, nil, nil, amadeus.NewLogger(nil, false))

	// then
	if err == nil {
		t.Fatal("expected error for empty RepoPath")
	}
	if got := err.Error(); got != "command validation: RepoPath is required" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestCollectPruneCandidates_InvalidCommand(t *testing.T) {
	// given: missing Days
	cmd := amadeus.ArchivePruneCommand{RepoPath: "/tmp/test", Days: 0}

	// when
	_, err := CollectPruneCandidates(cmd)

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
	cmd := amadeus.ArchivePruneCommand{RepoPath: "", Days: 30}

	// when
	_, err := CollectPruneCandidates(cmd)

	// then
	if err == nil {
		t.Fatal("expected error for empty RepoPath")
	}
	if got := err.Error(); got != "command validation: RepoPath is required" {
		t.Fatalf("unexpected error: %s", got)
	}
}
