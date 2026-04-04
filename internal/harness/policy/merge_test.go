package policy_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness/policy"
)

func mustPR(t *testing.T, number, title, base, head string, mergeable bool, labels []string, sha string) domain.PRState {
	t.Helper()
	pr, err := domain.NewPRState(number, title, base, head, mergeable, 0, nil, labels, sha)
	if err != nil {
		t.Fatalf("NewPRState: %v", err)
	}
	return pr
}

func TestDetermineMergeMethod_ChainRootWithDependents_ReturnsMerge(t *testing.T) {
	// given: a chain with root (#1 → main) and dependent (#2 → #1's branch)
	root := mustPR(t, "#1", "root", "main", "feat-a", true, nil, "abc123")
	leaf := mustPR(t, "#2", "leaf", "feat-a", "feat-b", true, nil, "def456")
	chain := domain.PRChain{ID: "chain-a", PRs: []domain.PRState{root, leaf}}

	// when
	method := policy.DetermineMergeMethod(root, &chain)

	// then: merge (not squash) to preserve hash for dependents
	if method != domain.MergeMethodMerge {
		t.Errorf("expected merge, got %s", method)
	}
}

func TestDetermineMergeMethod_ChainLeaf_ReturnsSquash(t *testing.T) {
	// given: leaf of a chain (no dependents after it)
	root := mustPR(t, "#1", "root", "main", "feat-a", true, nil, "abc123")
	leaf := mustPR(t, "#2", "leaf", "feat-a", "feat-b", true, nil, "def456")
	chain := domain.PRChain{ID: "chain-a", PRs: []domain.PRState{root, leaf}}

	// when
	method := policy.DetermineMergeMethod(leaf, &chain)

	// then: squash (clean history, no dependents)
	if method != domain.MergeMethodSquash {
		t.Errorf("expected squash, got %s", method)
	}
}

func TestDetermineMergeMethod_Standalone_ReturnsSquash(t *testing.T) {
	// given: standalone PR (nil chain)
	pr := mustPR(t, "#1", "solo", "main", "feat-a", true, nil, "abc123")

	// when
	method := policy.DetermineMergeMethod(pr, nil)

	// then
	if method != domain.MergeMethodSquash {
		t.Errorf("expected squash, got %s", method)
	}
}

func TestDetermineMergeMethod_SinglePRChain_ReturnsSquash(t *testing.T) {
	// given: chain with only 1 PR (no dependents)
	pr := mustPR(t, "#1", "solo", "main", "feat-a", true, nil, "abc123")
	chain := domain.PRChain{ID: "chain-a", PRs: []domain.PRState{pr}}

	// when
	method := policy.DetermineMergeMethod(pr, &chain)

	// then: squash (no dependents even though in a chain)
	if method != domain.MergeMethodSquash {
		t.Errorf("expected squash, got %s", method)
	}
}

func TestEvaluateMergeReadiness_AllGreen(t *testing.T) {
	// given: all preconditions met
	readiness := policy.EvaluateMergeReadiness(
		"#1", "CLEAN", "APPROVED", "MERGEABLE", true,
	)

	// then
	if !readiness.Ready {
		t.Errorf("expected Ready=true, got false; reasons: %v", readiness.BlockReasons)
	}
}

func TestEvaluateMergeReadiness_CIFailing(t *testing.T) {
	// given: merge state not clean
	readiness := policy.EvaluateMergeReadiness(
		"#1", "BLOCKED", "APPROVED", "MERGEABLE", true,
	)

	// then
	if readiness.Ready {
		t.Error("expected Ready=false for blocked CI")
	}
	if len(readiness.BlockReasons) == 0 {
		t.Error("expected BlockReasons to contain CI failure")
	}
}

func TestEvaluateMergeReadiness_ReviewRequired(t *testing.T) {
	// given: review not approved
	readiness := policy.EvaluateMergeReadiness(
		"#1", "CLEAN", "REVIEW_REQUIRED", "MERGEABLE", true,
	)

	// then
	if readiness.Ready {
		t.Error("expected Ready=false for review required")
	}
}

func TestEvaluateMergeReadiness_NoReviewDecision_IsOK(t *testing.T) {
	// given: no reviewers assigned (empty review decision)
	readiness := policy.EvaluateMergeReadiness(
		"#1", "CLEAN", "", "MERGEABLE", true,
	)

	// then: empty review decision = no reviewers = OK
	if !readiness.Ready {
		t.Errorf("expected Ready=true for empty reviewDecision, got false; reasons: %v", readiness.BlockReasons)
	}
}

func TestEvaluateMergeReadiness_NotMergeable(t *testing.T) {
	// given: conflicting
	readiness := policy.EvaluateMergeReadiness(
		"#1", "CLEAN", "APPROVED", "CONFLICTING", true,
	)

	// then
	if readiness.Ready {
		t.Error("expected Ready=false for conflicting")
	}
}

func TestEvaluateMergeReadiness_NoReviewLabel(t *testing.T) {
	// given: amadeus hasn't reviewed
	readiness := policy.EvaluateMergeReadiness(
		"#1", "CLEAN", "APPROVED", "MERGEABLE", false,
	)

	// then
	if readiness.Ready {
		t.Error("expected Ready=false for missing review label")
	}
}

func TestFilterMergeReady(t *testing.T) {
	// given
	readyPR := policy.EvaluateMergeReadiness("#1", "CLEAN", "APPROVED", "MERGEABLE", true)
	blockedPR := policy.EvaluateMergeReadiness("#2", "BLOCKED", "APPROVED", "MERGEABLE", true)

	// when
	ready := policy.FilterMergeReady([]domain.PRMergeReadiness{readyPR, blockedPR})

	// then
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready PR, got %d", len(ready))
	}
	if ready[0].Number != "#1" {
		t.Errorf("expected #1, got %s", ready[0].Number)
	}
}

func TestEvaluateMergeReadiness_ChangesRequested(t *testing.T) {
	readiness := policy.EvaluateMergeReadiness("#1", "CLEAN", "CHANGES_REQUESTED", "MERGEABLE", true)
	if readiness.Ready {
		t.Error("expected Ready=false for CHANGES_REQUESTED")
	}
}

func TestEvaluateMergeReadiness_MultipleBlockReasons(t *testing.T) {
	// given: all 4 conditions fail
	readiness := policy.EvaluateMergeReadiness("#1", "BLOCKED", "REVIEW_REQUIRED", "CONFLICTING", false)

	// then
	if readiness.Ready {
		t.Error("expected Ready=false")
	}
	if len(readiness.BlockReasons) != 4 {
		t.Errorf("expected 4 block reasons, got %d: %v", len(readiness.BlockReasons), readiness.BlockReasons)
	}
}

// TestGoTaskboardScenario_ChainDetectionAndMergeStrategy reproduces the
// exact go-taskboard state (2026-03-30) to verify chain detection and
// merge strategy selection work correctly with real-world data.
//
// go-taskboard state:
// - 17 standalone PRs targeting main (all MERGEABLE, all reviewed)
// - 1 chain: #22 (base=main) → #23 (base=feat/...-http-test-infra)
// - #23 has NO amadeus review label
func TestGoTaskboardScenario_ChainDetectionAndMergeStrategy(t *testing.T) {
	// given: reproduce go-taskboard PR set
	prs := []domain.PRState{
		mustPR(t, "#14", "input-validation-status", "main", "feat/input-validation-cluster-w2-5", true, []string{"amadeus:reviewed-a9c5e6e3"}, "a9c5e6e3"),
		mustPR(t, "#15", "pagination-reproduction", "main", "feat/pagination-w1-s1", true, []string{"amadeus:reviewed-e6a3617a"}, "e6a3617a"),
		mustPR(t, "#16", "handler-validation", "main", "feat/cluster-w2-1-handler", true, []string{"amadeus:reviewed-411f4200"}, "411f4200"),
		mustPR(t, "#17", "acceptance-tests", "main", "feat/input-validation-cluster-w3-5", true, []string{"amadeus:reviewed-0ba57d71"}, "0ba57d71"),
		mustPR(t, "#18", "pagination-investigation", "main", "feat/pagination-cluster-w1-1", true, []string{"amadeus:reviewed-e6b39c3a"}, "e6b39c3a"),
		mustPR(t, "#19", "input-validation", "main", "feat/pagination-w2-1-input", true, []string{"amadeus:reviewed-b04b4e5c"}, "b04b4e5c"),
		mustPR(t, "#20", "sqlite-filter", "main", "feat/task-api-cluster-w2-12", true, []string{"amadeus:reviewed-457e717f"}, "457e717f"),
		// Chain: #22 → #23
		mustPR(t, "#22", "http-test-infra", "main", "feat/pagination-validation-w1-21-http-test-infra", true, []string{"amadeus:reviewed-0ef92367"}, "0ef92367"),
		mustPR(t, "#23", "reproduction-test", "feat/pagination-validation-w1-21-http-test-infra", "feat/pagination-validation-w3-1", true, nil, "deadbeef"), // no review label
		// More standalone
		mustPR(t, "#24", "offset-limit", "main", "feat/pagination-validation-w2-2", true, []string{"amadeus:reviewed-52a3b666"}, "52a3b666"),
		mustPR(t, "#25", "handler-boundary", "main", "feat/pagination-validation-w2-3", true, []string{"amadeus:reviewed-40b2ac38"}, "40b2ac38"),
		mustPR(t, "#26", "status-validation", "main", "feat/cluster-w1-5", true, []string{"amadeus:reviewed-0d23ccee"}, "0d23ccee"),
		mustPR(t, "#27", "export-is-valid", "main", "feat/cluster-w2-1-6-export", true, []string{"amadeus:reviewed-3e8455a6"}, "3e8455a6"),
		mustPR(t, "#28", "export-set-priority", "main", "feat/cluster-w2-1-6-export-set", true, []string{"amadeus:reviewed-57350364"}, "57350364"),
		mustPR(t, "#29", "status-filter", "main", "feat/cluster-w3-7", true, []string{"amadeus:reviewed-56cbe9db"}, "56cbe9db"),
		mustPR(t, "#30", "stats-endpoint", "main", "feat/cluster-w3-8-stats", true, []string{"amadeus:reviewed-3fade125"}, "3fade125"),
		mustPR(t, "#31", "errors-is-migration", "main", "expedition/052", true, []string{"amadeus:reviewed-33f559fb"}, "33f559fb"),
		mustPR(t, "#32", "zero-count", "main", "feat/cluster-w1-8-zero", true, []string{"amadeus:reviewed-ce8a5bbe"}, "ce8a5bbe"),
	}

	// when: build convergence report
	report := policy.BuildPRConvergenceReport("main", prs)

	// then: should detect exactly 1 chain (#22 → #23)
	chainCount := 0
	var chainWith22 *domain.PRChain
	for i, chain := range report.Chains {
		if len(chain.PRs) > 1 {
			chainCount++
			chainWith22 = &report.Chains[i]
		}
	}
	if chainCount != 1 {
		t.Fatalf("expected 1 multi-PR chain, got %d", chainCount)
	}
	if len(chainWith22.PRs) != 2 {
		t.Fatalf("expected chain length 2, got %d", len(chainWith22.PRs))
	}
	if chainWith22.PRs[0].Number() != "#22" {
		t.Errorf("expected chain root=#22, got %s", chainWith22.PRs[0].Number())
	}
	if chainWith22.PRs[1].Number() != "#23" {
		t.Errorf("expected chain leaf=#23, got %s", chainWith22.PRs[1].Number())
	}

	// then: #22 (chain root with dependent) → merge method
	method22 := policy.DetermineMergeMethod(chainWith22.PRs[0], chainWith22)
	if method22 != domain.MergeMethodMerge {
		t.Errorf("#22 (chain root): expected merge, got %s", method22)
	}

	// then: #23 (chain leaf) → squash method
	method23 := policy.DetermineMergeMethod(chainWith22.PRs[1], chainWith22)
	if method23 != domain.MergeMethodSquash {
		t.Errorf("#23 (chain leaf): expected squash, got %s", method23)
	}

	// then: standalone PRs → squash
	for _, chain := range report.Chains {
		if len(chain.PRs) == 1 {
			method := policy.DetermineMergeMethod(chain.PRs[0], &chain)
			if method != domain.MergeMethodSquash {
				t.Errorf("%s (standalone): expected squash, got %s", chain.PRs[0].Number(), method)
			}
		}
	}

	// then: #23 without review label → not merge ready
	readiness23 := policy.EvaluateMergeReadiness("#23", "CLEAN", "", "MERGEABLE", false)
	if readiness23.Ready {
		t.Error("#23 should NOT be merge-ready (no review label)")
	}

	// then: #22 with review label → merge ready
	readiness22 := policy.EvaluateMergeReadiness("#22", "CLEAN", "", "MERGEABLE", true)
	if !readiness22.Ready {
		t.Errorf("#22 should be merge-ready, got reasons: %v", readiness22.BlockReasons)
	}
}
