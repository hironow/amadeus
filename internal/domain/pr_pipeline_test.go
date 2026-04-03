package domain_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

// --- IsPipelinePR tests ---

func TestIsPipelinePR_PaintressPROpenLabel(t *testing.T) {
	// given: PR with paintress:pr-open label
	pr, err := domain.NewPRState("#23", "test PR", "some-branch", "feat/test", true, 0, nil,
		[]string{"paintress:pr-open"}, "abc12345")
	if err != nil {
		t.Fatal(err)
	}

	// when
	result := domain.IsPipelinePR(pr)

	// then
	if !result {
		t.Error("expected IsPipelinePR=true for paintress:pr-open label")
	}
}

func TestIsPipelinePR_SightjackReadyLabel(t *testing.T) {
	// given: PR with sightjack:ready label
	pr, err := domain.NewPRState("#10", "feat PR", "some-branch", "feat/test", true, 0, nil,
		[]string{"sightjack:ready"}, "abc12345")
	if err != nil {
		t.Fatal(err)
	}

	// when
	result := domain.IsPipelinePR(pr)

	// then
	if !result {
		t.Error("expected IsPipelinePR=true for sightjack:ready label")
	}
}

func TestIsPipelinePR_WavePatternInHeadBranch(t *testing.T) {
	tests := []struct {
		name   string
		head   string
		expect bool
	}{
		{"cluster wave", "feat/cluster-w1-21-list-response", true},
		{"pagination wave", "feat/pagination-validation-w3-1-reproduction-test", true},
		{"simple wave", "feat/task-api-cluster-w2-12-sqlite-filter", true},
		{"no wave", "feat/some-normal-branch", false},
		{"wave-like but not pattern", "feat/www-website", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			pr, err := domain.NewPRState("#1", "test", "main", tt.head, true, 0, nil, nil, "abc12345")
			if err != nil {
				t.Fatal(err)
			}

			// when
			result := domain.IsPipelinePR(pr)

			// then
			if result != tt.expect {
				t.Errorf("IsPipelinePR for head=%q: got %v, want %v", tt.head, result, tt.expect)
			}
		})
	}
}

func TestIsPipelinePR_WavePatternInBaseBranch(t *testing.T) {
	// given: PR #23 scenario — base branch is a wave pattern (merged feature branch)
	pr, err := domain.NewPRState("#23", "test", "feat/pagination-validation-w1-21-http-test-infra",
		"feat/pagination-validation-w3-1-reproduction-test", true, 0, nil, nil, "abc12345")
	if err != nil {
		t.Fatal(err)
	}

	// when
	result := domain.IsPipelinePR(pr)

	// then
	if !result {
		t.Error("expected IsPipelinePR=true when base branch has wave pattern")
	}
}

func TestIsPipelinePR_ExpeditionBranch(t *testing.T) {
	// given: expedition branch from paintress
	pr, err := domain.NewPRState("#31", "feat: migrate", "main", "expedition/052-errors-is-migration",
		true, 0, nil, nil, "abc12345")
	if err != nil {
		t.Fatal(err)
	}

	// when
	result := domain.IsPipelinePR(pr)

	// then
	if !result {
		t.Error("expected IsPipelinePR=true for expedition/ branch")
	}
}

func TestIsPipelinePR_AmadeusFixBranch(t *testing.T) {
	tests := []struct {
		name string
		head string
	}{
		{"am-pr-review", "feat/am-pr-review-35-23c69edc-2-omitempty-json-tags"},
		{"am-implementation-feedback", "feat/am-implementation-feedback-037_ae6331b4-stats"},
		{"am-conflict", "feat/am-conflict-14-status-validation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			pr, err := domain.NewPRState("#1", "fix", "main", tt.head, true, 0, nil, nil, "abc12345")
			if err != nil {
				t.Fatal(err)
			}

			// when
			result := domain.IsPipelinePR(pr)

			// then
			if !result {
				t.Errorf("expected IsPipelinePR=true for head=%q", tt.head)
			}
		})
	}
}

func TestIsPipelinePR_NormalPR(t *testing.T) {
	// given: a PR with no pipeline indicators
	pr, err := domain.NewPRState("#99", "chore: update deps", "main", "chore/update-deps",
		true, 0, nil, nil, "abc12345")
	if err != nil {
		t.Fatal(err)
	}

	// when
	result := domain.IsPipelinePR(pr)

	// then
	if result {
		t.Error("expected IsPipelinePR=false for normal PR with no pipeline indicators")
	}
}

// --- IsPipelinePRWithIssueContext tests ---

func TestIsPipelinePRWithIssueContext_MatchesTitleIssue(t *testing.T) {
	// given: PR title references #1, and issue 1 has sightjack:ready
	pr, err := domain.NewPRState("#23", "test: #1 pagination bug reproduction",
		"some-old-branch", "fix/some-branch", true, 0, nil, nil, "abc12345")
	if err != nil {
		t.Fatal(err)
	}
	sightjackIssues := []string{"1", "5", "9"}

	// when: IsPipelinePR returns false (no label/branch match)
	if domain.IsPipelinePR(pr) {
		t.Skip("IsPipelinePR already true — this test targets issue-link fallback")
	}

	// when
	result := domain.IsPipelinePRWithIssueContext(pr, sightjackIssues)

	// then
	if !result {
		t.Error("expected true: PR title references issue #1 which has sightjack:ready")
	}
}

func TestIsPipelinePRWithIssueContext_NoMatchingIssue(t *testing.T) {
	// given: PR title references #99, but only issues 1,5,9 have sightjack:ready
	pr, err := domain.NewPRState("#50", "fix: unrelated #99",
		"some-old-branch", "fix/unrelated", true, 0, nil, nil, "abc12345")
	if err != nil {
		t.Fatal(err)
	}
	sightjackIssues := []string{"1", "5", "9"}

	// when
	result := domain.IsPipelinePRWithIssueContext(pr, sightjackIssues)

	// then
	if result {
		t.Error("expected false: PR references #99 which is not a sightjack issue")
	}
}

func TestIsPipelinePRWithIssueContext_NilIssueList(t *testing.T) {
	// given: no sightjack issues available
	pr, err := domain.NewPRState("#23", "test: #1 bug",
		"old-branch", "fix/test", true, 0, nil, nil, "abc12345")
	if err != nil {
		t.Fatal(err)
	}

	// when
	result := domain.IsPipelinePRWithIssueContext(pr, nil)

	// then
	if result {
		t.Error("expected false: no sightjack issues to match against")
	}
}

func TestIsPipelinePRWithIssueContext_MultipleIssuesInTitle(t *testing.T) {
	// given: PR title references #2 and #3, issue 3 has sightjack:ready
	pr, err := domain.NewPRState("#24", "fix: issues #2, #3 validation",
		"old-branch", "fix/validation", true, 0, nil, nil, "abc12345")
	if err != nil {
		t.Fatal(err)
	}
	sightjackIssues := []string{"3", "7"}

	// when
	result := domain.IsPipelinePRWithIssueContext(pr, sightjackIssues)

	// then
	if !result {
		t.Error("expected true: PR references #3 which has sightjack:ready")
	}
}

// --- ExtractGitHubIssueNumbers tests ---

func TestExtractGitHubIssueNumbers_FromTitle(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected []string
	}{
		{"hash prefix", "test: #1 pagination bug reproduction tests", []string{"1"}},
		{"parenthesized", "test: add response body verification (#21)", []string{"21"}},
		{"test prefix", "test(#6): E2E tests for UpdateTask API", []string{"6"}},
		{"multiple", "fix: issues #2, #3 offset/limit validation", []string{"2", "3"}},
		{"none", "chore: update dependencies", nil},
		{"ADR reference not issue", "feat: wire stats (ADR-0014)", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			result := domain.ExtractGitHubIssueNumbers(tt.title)

			// then
			if len(result) != len(tt.expected) {
				t.Fatalf("got %v, want %v", result, tt.expected)
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("result[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}
