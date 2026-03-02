package session

import (
	"testing"

	amadeus "github.com/hironow/amadeus"
)

func TestParseMergedPRs_MergeCommit(t *testing.T) {
	// Standard GitHub merge commit format
	log := "abc1234 Merge pull request #42 from user/feature-branch"

	prs := parseMergedPRs(log)

	if len(prs) != 1 {
		t.Fatalf("got %d PRs, want 1", len(prs))
	}
	if prs[0].Number != "#42" {
		t.Errorf("Number = %q, want %q", prs[0].Number, "#42")
	}
}

func TestParseMergedPRs_SquashMerge(t *testing.T) {
	// GitHub squash merge format: title (#NNN)
	log := "def5678 Add authentication feature (#123)"

	prs := parseMergedPRs(log)

	if len(prs) != 1 {
		t.Fatalf("got %d PRs, want 1", len(prs))
	}
	if prs[0].Number != "#123" {
		t.Errorf("Number = %q, want %q", prs[0].Number, "#123")
	}
}

func TestParseMergedPRs_Mixed(t *testing.T) {
	log := `abc1234 Merge pull request #42 from user/feature
def5678 Fix bug in login (#99)
ghi9012 Update README (#7)`

	prs := parseMergedPRs(log)

	if len(prs) != 3 {
		t.Fatalf("got %d PRs, want 3", len(prs))
	}
	want := []string{"#42", "#99", "#7"}
	for i, pr := range prs {
		if pr.Number != want[i] {
			t.Errorf("prs[%d].Number = %q, want %q", i, pr.Number, want[i])
		}
	}
}

func TestParseMergedPRs_NoPRs(t *testing.T) {
	log := `abc1234 chore: update deps
def5678 fix typo in docs`

	prs := parseMergedPRs(log)

	if len(prs) != 0 {
		t.Errorf("got %d PRs, want 0", len(prs))
	}
}

func TestParseMergedPRs_Empty(t *testing.T) {
	prs := parseMergedPRs("")

	if len(prs) != 0 {
		t.Errorf("got %d PRs, want 0", len(prs))
	}
}

func TestParseMergedPRs_NoDuplicates(t *testing.T) {
	// A merge commit that also has (#NNN) in the title should not produce duplicates
	log := "abc1234 Merge pull request #42 from user/feat (#42)"

	prs := parseMergedPRs(log)

	if len(prs) != 1 {
		t.Fatalf("got %d PRs, want 1 (no duplicates)", len(prs))
	}
	if prs[0].Number != "#42" {
		t.Errorf("Number = %q, want %q", prs[0].Number, "#42")
	}
}

// Verify parseMergedPRs returns correct type.
var _ []amadeus.MergedPR = parseMergedPRs("")
