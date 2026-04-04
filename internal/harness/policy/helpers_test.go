package policy_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func mustPRState(t *testing.T, number, title, baseBranch, headBranch string, mergeable bool, behindBy int, conflictFiles []string) domain.PRState {
	t.Helper()
	ps, err := domain.NewPRState(number, title, baseBranch, headBranch, mergeable, behindBy, conflictFiles, nil, "")
	if err != nil {
		t.Fatalf("mustPRState: %v", err)
	}
	return ps
}
