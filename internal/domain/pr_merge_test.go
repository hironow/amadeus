package domain_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
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
	method := domain.DetermineMergeMethod(root, &chain)

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
	method := domain.DetermineMergeMethod(leaf, &chain)

	// then: squash (clean history, no dependents)
	if method != domain.MergeMethodSquash {
		t.Errorf("expected squash, got %s", method)
	}
}

func TestDetermineMergeMethod_Standalone_ReturnsSquash(t *testing.T) {
	// given: standalone PR (nil chain)
	pr := mustPR(t, "#1", "solo", "main", "feat-a", true, nil, "abc123")

	// when
	method := domain.DetermineMergeMethod(pr, nil)

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
	method := domain.DetermineMergeMethod(pr, &chain)

	// then: squash (no dependents even though in a chain)
	if method != domain.MergeMethodSquash {
		t.Errorf("expected squash, got %s", method)
	}
}

func TestEvaluateMergeReadiness_AllGreen(t *testing.T) {
	// given: all preconditions met
	readiness := domain.EvaluateMergeReadiness(
		"#1", "CLEAN", "APPROVED", "MERGEABLE", true,
	)

	// then
	if !readiness.Ready {
		t.Errorf("expected Ready=true, got false; reasons: %v", readiness.BlockReasons)
	}
}

func TestEvaluateMergeReadiness_CIFailing(t *testing.T) {
	// given: merge state not clean
	readiness := domain.EvaluateMergeReadiness(
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
	readiness := domain.EvaluateMergeReadiness(
		"#1", "CLEAN", "REVIEW_REQUIRED", "MERGEABLE", true,
	)

	// then
	if readiness.Ready {
		t.Error("expected Ready=false for review required")
	}
}

func TestEvaluateMergeReadiness_NoReviewDecision_IsOK(t *testing.T) {
	// given: no reviewers assigned (empty review decision)
	readiness := domain.EvaluateMergeReadiness(
		"#1", "CLEAN", "", "MERGEABLE", true,
	)

	// then: empty review decision = no reviewers = OK
	if !readiness.Ready {
		t.Errorf("expected Ready=true for empty reviewDecision, got false; reasons: %v", readiness.BlockReasons)
	}
}

func TestEvaluateMergeReadiness_NotMergeable(t *testing.T) {
	// given: conflicting
	readiness := domain.EvaluateMergeReadiness(
		"#1", "CLEAN", "APPROVED", "CONFLICTING", true,
	)

	// then
	if readiness.Ready {
		t.Error("expected Ready=false for conflicting")
	}
}

func TestEvaluateMergeReadiness_NoReviewLabel(t *testing.T) {
	// given: amadeus hasn't reviewed
	readiness := domain.EvaluateMergeReadiness(
		"#1", "CLEAN", "APPROVED", "MERGEABLE", false,
	)

	// then
	if readiness.Ready {
		t.Error("expected Ready=false for missing review label")
	}
}

func TestFilterMergeReady(t *testing.T) {
	// given
	readyPR := domain.EvaluateMergeReadiness("#1", "CLEAN", "APPROVED", "MERGEABLE", true)
	blockedPR := domain.EvaluateMergeReadiness("#2", "BLOCKED", "APPROVED", "MERGEABLE", true)

	// when
	ready := domain.FilterMergeReady([]domain.PRMergeReadiness{readyPR, blockedPR})

	// then
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready PR, got %d", len(ready))
	}
	if ready[0].Number != "#1" {
		t.Errorf("expected #1, got %s", ready[0].Number)
	}
}
