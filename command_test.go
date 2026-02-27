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
