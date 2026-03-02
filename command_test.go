package amadeus_test

import (
	"testing"

	"github.com/hironow/amadeus"
)

func TestExecuteCheckCommand_Validate_Valid(t *testing.T) {
	// given
	cmd := amadeus.ExecuteCheckCommand{
		RepoPath: "/tmp/repo",
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestExecuteCheckCommand_Validate_MissingRepoPath(t *testing.T) {
	// given
	cmd := amadeus.ExecuteCheckCommand{}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) == 0 {
		t.Fatal("expected validation error for missing RepoPath")
	}
}

func TestRunSyncCommand_Validate_Valid(t *testing.T) {
	// given
	cmd := amadeus.RunSyncCommand{
		RepoPath: "/tmp/repo",
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestRunSyncCommand_Validate_MissingRepoPath(t *testing.T) {
	// given
	cmd := amadeus.RunSyncCommand{}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) == 0 {
		t.Fatal("expected validation error for missing RepoPath")
	}
}

func TestRebuildCommand_Validate_Valid(t *testing.T) {
	// given
	cmd := amadeus.RebuildCommand{
		RepoPath: "/tmp/repo",
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestRebuildCommand_Validate_MissingRepoPath(t *testing.T) {
	// given
	cmd := amadeus.RebuildCommand{}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) == 0 {
		t.Fatal("expected validation error for missing RepoPath")
	}
}

func TestArchivePruneCommand_Validate_Valid(t *testing.T) {
	// given
	cmd := amadeus.ArchivePruneCommand{
		RepoPath: "/tmp/repo",
		Days:     30,
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestArchivePruneCommand_Validate_InvalidDays(t *testing.T) {
	// given
	cmd := amadeus.ArchivePruneCommand{
		RepoPath: "/tmp/repo",
		Days:     0,
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) == 0 {
		t.Fatal("expected validation error for non-positive Days")
	}
}
