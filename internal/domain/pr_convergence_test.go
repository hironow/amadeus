package domain_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// --- PRState constructor tests ---

func TestNewPRState_valid(t *testing.T) {
	// given
	ps, err := domain.NewPRState("#42", "Fix bug", "main", "feature/fix", true, 3, []string{"file.go"}, nil, "")

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ps.Number() != "#42" {
		t.Errorf("expected #42, got %q", ps.Number())
	}
	if ps.Title() != "Fix bug" {
		t.Errorf("expected Fix bug, got %q", ps.Title())
	}
	if ps.BaseBranch() != "main" {
		t.Errorf("expected main, got %q", ps.BaseBranch())
	}
	if ps.HeadBranch() != "feature/fix" {
		t.Errorf("expected feature/fix, got %q", ps.HeadBranch())
	}
	if !ps.Mergeable() {
		t.Error("expected mergeable true")
	}
	if ps.BehindBy() != 3 {
		t.Errorf("expected behindBy 3, got %d", ps.BehindBy())
	}
	if !ps.HasConflict() {
		t.Error("expected HasConflict true when conflictFiles non-empty")
	}
	if len(ps.ConflictFiles()) != 1 || ps.ConflictFiles()[0] != "file.go" {
		t.Errorf("expected [file.go], got %v", ps.ConflictFiles())
	}
}

func TestNewPRState_emptyNumber_fails(t *testing.T) {
	// when
	_, err := domain.NewPRState("", "title", "main", "feat", true, 0, nil, nil, "")

	// then
	if err == nil {
		t.Fatal("expected error for empty number")
	}
}

func TestNewPRState_emptyBaseBranch_fails(t *testing.T) {
	// when
	_, err := domain.NewPRState("#1", "title", "", "feat", true, 0, nil, nil, "")

	// then
	if err == nil {
		t.Fatal("expected error for empty baseBranch")
	}
}

func TestNewPRState_emptyHeadBranch_fails(t *testing.T) {
	// when
	_, err := domain.NewPRState("#1", "title", "main", "", true, 0, nil, nil, "")

	// then
	if err == nil {
		t.Fatal("expected error for empty headBranch")
	}
}

func TestNewPRState_noConflict(t *testing.T) {
	// given
	ps, err := domain.NewPRState("#1", "title", "main", "feat", true, 0, nil, nil, "")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps.HasConflict() {
		t.Error("expected HasConflict false when conflictFiles is nil")
	}
}

// --- BuildPRConvergenceReport tests ---

func TestBuildPRChains_singleChain(t *testing.T) {
	// given: 3 PRs forming a single chain main <- feat-a <- feat-b <- feat-c
	pr1 := mustPRState(t, "#1", "A", "main", "feat-a", true, 0, nil)
	pr2 := mustPRState(t, "#2", "B", "feat-a", "feat-b", true, 0, nil)
	pr3 := mustPRState(t, "#3", "C", "feat-b", "feat-c", true, 0, nil)

	// when
	report := domain.BuildPRConvergenceReport("main", []domain.PRState{pr1, pr2, pr3})

	// then
	if len(report.Chains) != 1 {
		t.Fatalf("expected 1 chain, got %d", len(report.Chains))
	}
	if len(report.Chains[0].PRs) != 3 {
		t.Errorf("expected 3 PRs in chain, got %d", len(report.Chains[0].PRs))
	}
	if report.Chains[0].ID != "chain-a" {
		t.Errorf("expected chain-a, got %q", report.Chains[0].ID)
	}
	if report.Chains[0].HasConflict {
		t.Error("expected no conflict in chain")
	}
	if len(report.OrphanedPRs) != 0 {
		t.Errorf("expected 0 orphaned PRs, got %d", len(report.OrphanedPRs))
	}
	if report.TotalOpenPRs != 3 {
		t.Errorf("expected TotalOpenPRs 3, got %d", report.TotalOpenPRs)
	}
	if report.IntegrationBranch != "main" {
		t.Errorf("expected integration branch main, got %q", report.IntegrationBranch)
	}
}

func TestBuildPRChains_multipleChains(t *testing.T) {
	// given: 3 PRs forming 2 chains
	// chain-a: main <- feat-a <- feat-b
	// chain-b: main <- feat-x
	pr1 := mustPRState(t, "#1", "A", "main", "feat-a", true, 0, nil)
	pr2 := mustPRState(t, "#2", "B", "feat-a", "feat-b", true, 0, nil)
	pr3 := mustPRState(t, "#3", "X", "main", "feat-x", true, 0, nil)

	// when
	report := domain.BuildPRConvergenceReport("main", []domain.PRState{pr1, pr2, pr3})

	// then
	if len(report.Chains) != 2 {
		t.Fatalf("expected 2 chains, got %d", len(report.Chains))
	}
	if report.Chains[0].ID != "chain-a" {
		t.Errorf("expected chain-a, got %q", report.Chains[0].ID)
	}
	if report.Chains[1].ID != "chain-b" {
		t.Errorf("expected chain-b, got %q", report.Chains[1].ID)
	}
	if len(report.OrphanedPRs) != 0 {
		t.Errorf("expected 0 orphaned PRs, got %d", len(report.OrphanedPRs))
	}
}

func TestBuildPRChains_orphanedPR(t *testing.T) {
	// given: PR whose base branch is not reachable from integration branch
	pr1 := mustPRState(t, "#1", "A", "main", "feat-a", true, 0, nil)
	pr2 := mustPRState(t, "#2", "Orphan", "some-other-branch", "feat-orphan", true, 0, nil)

	// when
	report := domain.BuildPRConvergenceReport("main", []domain.PRState{pr1, pr2})

	// then
	if len(report.Chains) != 1 {
		t.Fatalf("expected 1 chain, got %d", len(report.Chains))
	}
	if len(report.OrphanedPRs) != 1 {
		t.Fatalf("expected 1 orphaned PR, got %d", len(report.OrphanedPRs))
	}
	if report.OrphanedPRs[0].Number() != "#2" {
		t.Errorf("expected orphaned PR #2, got %q", report.OrphanedPRs[0].Number())
	}
	if report.TotalOpenPRs != 2 {
		t.Errorf("expected TotalOpenPRs 2, got %d", report.TotalOpenPRs)
	}
}

func TestBuildPRChains_conflictDetection(t *testing.T) {
	// given: chain with a conflicting PR
	pr1 := mustPRState(t, "#1", "A", "main", "feat-a", true, 0, nil)
	pr2 := mustPRState(t, "#2", "B", "feat-a", "feat-b", false, 5, []string{"conflict.go"})

	// when
	report := domain.BuildPRConvergenceReport("main", []domain.PRState{pr1, pr2})

	// then
	if len(report.Chains) != 1 {
		t.Fatalf("expected 1 chain, got %d", len(report.Chains))
	}
	if !report.Chains[0].HasConflict {
		t.Error("expected chain to have conflict")
	}
}

func TestBuildPRChains_branchingChain(t *testing.T) {
	// given: main <- feat-a, feat-a <- feat-b, feat-a <- feat-c (diamond shape)
	// BFS guarantees feat-a comes before both feat-b and feat-c.
	pr1 := mustPRState(t, "#1", "A", "main", "feat-a", true, 0, nil)
	pr2 := mustPRState(t, "#2", "B", "feat-a", "feat-b", true, 0, nil)
	pr3 := mustPRState(t, "#3", "C", "feat-a", "feat-c", true, 0, nil)

	// when
	report := domain.BuildPRConvergenceReport("main", []domain.PRState{pr1, pr2, pr3})

	// then: single chain with all 3 PRs
	if len(report.Chains) != 1 {
		t.Fatalf("expected 1 chain, got %d", len(report.Chains))
	}
	chain := report.Chains[0]
	if len(chain.PRs) != 3 {
		t.Fatalf("expected 3 PRs in chain, got %d", len(chain.PRs))
	}
	// feat-a (root) must be first; feat-b and feat-c follow in either order.
	if chain.PRs[0].Number() != "#1" {
		t.Errorf("expected first PR to be #1 (root), got %q", chain.PRs[0].Number())
	}
	// Both dependents must appear after the root.
	dependents := map[string]bool{
		chain.PRs[1].Number(): true,
		chain.PRs[2].Number(): true,
	}
	if !dependents["#2"] || !dependents["#3"] {
		t.Errorf("expected dependents {#2, #3}, got {%q, %q}", chain.PRs[1].Number(), chain.PRs[2].Number())
	}
	if len(report.OrphanedPRs) != 0 {
		t.Errorf("expected 0 orphaned PRs, got %d", len(report.OrphanedPRs))
	}
}

func TestBuildPRChains_emptyPRs(t *testing.T) {
	// when
	report := domain.BuildPRConvergenceReport("main", nil)

	// then
	if len(report.Chains) != 0 {
		t.Errorf("expected 0 chains, got %d", len(report.Chains))
	}
	if len(report.OrphanedPRs) != 0 {
		t.Errorf("expected 0 orphaned PRs, got %d", len(report.OrphanedPRs))
	}
	if report.TotalOpenPRs != 0 {
		t.Errorf("expected TotalOpenPRs 0, got %d", report.TotalOpenPRs)
	}
	if report.IntegrationBranch != "main" {
		t.Errorf("expected integration branch main, got %q", report.IntegrationBranch)
	}
}

func TestBuildPRChains_manyPRsMixedChainsAndOrphans(t *testing.T) {
	// given: 30 PRs — some forming chains off "main", rest orphaned (targeting "develop")
	mustPR := func(num int, base, head string) domain.PRState {
		ps, err := domain.NewPRState(fmt.Sprintf("#%d", num), fmt.Sprintf("PR %d", num), base, head, true, 0, nil, nil, "")
		if err != nil {
			t.Fatalf("NewPRState #%d: %v", num, err)
		}
		return ps
	}

	var prs []domain.PRState
	// Chain: main <- a <- b <- c <- d <- e (5 deep)
	prs = append(prs,
		mustPR(1, "main", "feat-a"),
		mustPR(2, "feat-a", "feat-b"),
		mustPR(3, "feat-b", "feat-c"),
		mustPR(4, "feat-c", "feat-d"),
		mustPR(5, "feat-d", "feat-e"),
	)
	// 25 orphaned PRs targeting "develop"
	for i := 6; i <= 30; i++ {
		prs = append(prs, mustPR(i, "develop", fmt.Sprintf("orphan-%d", i)))
	}

	// when
	report := domain.BuildPRConvergenceReport("main", prs)

	// then
	if report.TotalOpenPRs != 30 {
		t.Errorf("TotalOpenPRs = %d, want 30", report.TotalOpenPRs)
	}
	// 1 chain (5 PRs deep)
	if len(report.Chains) != 1 {
		t.Fatalf("expected 1 chain, got %d", len(report.Chains))
	}
	if len(report.Chains[0].PRs) != 5 {
		t.Errorf("chain depth = %d, want 5", len(report.Chains[0].PRs))
	}
	// 25 orphaned
	if len(report.OrphanedPRs) != 25 {
		t.Errorf("orphaned = %d, want 25", len(report.OrphanedPRs))
	}
}

// --- ClassifyConvergenceScenario tests ---

func TestClassifyConvergenceScenario(t *testing.T) {
	tests := []struct {
		name             string
		chain            domain.PRChain
		expectedSeverity domain.Severity
		expectedAction   domain.DMailAction
	}{
		{
			name: "single PR, no conflict, behind > 0 = low severity, retry",
			chain: domain.PRChain{
				ID:          "chain-a",
				PRs:         []domain.PRState{mustPRState(t, "#1", "A", "main", "feat", true, 2, nil)},
				HasConflict: false,
			},
			expectedSeverity: domain.SeverityLow,
			expectedAction:   domain.ActionRetry,
		},
		{
			name: "chain >1 PR, no conflict = medium severity, retry",
			chain: domain.PRChain{
				ID: "chain-b",
				PRs: []domain.PRState{
					mustPRState(t, "#1", "A", "main", "feat-a", true, 0, nil),
					mustPRState(t, "#2", "B", "feat-a", "feat-b", true, 0, nil),
				},
				HasConflict: false,
			},
			expectedSeverity: domain.SeverityMedium,
			expectedAction:   domain.ActionRetry,
		},
		{
			name: "any conflict in chain = high severity, retry",
			chain: domain.PRChain{
				ID: "chain-c",
				PRs: []domain.PRState{
					mustPRState(t, "#1", "A", "main", "feat-a", true, 0, nil),
					mustPRState(t, "#2", "B", "feat-a", "feat-b", false, 3, []string{"x.go"}),
				},
				HasConflict: true,
			},
			expectedSeverity: domain.SeverityHigh,
			expectedAction:   domain.ActionRetry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			severity, action := domain.ClassifyConvergenceScenario(tt.chain)

			// then
			if severity != tt.expectedSeverity {
				t.Errorf("expected severity %q, got %q", tt.expectedSeverity, severity)
			}
			if action != tt.expectedAction {
				t.Errorf("expected action %q, got %q", tt.expectedAction, action)
			}
		})
	}
}

func TestBuildPRChains_circularDependency_terminates(t *testing.T) {
	// given: circular PR dependency: main <- feat-a <- feat-b <- feat-a (cycle)
	// This MUST terminate without infinite loop. Without cycle detection in
	// buildChainBFS, the queue grows unbounded and the test hangs.
	pr1 := mustPRState(t, "#1", "A", "main", "feat-a", true, 0, nil)
	pr2 := mustPRState(t, "#2", "B", "feat-a", "feat-b", true, 0, nil)
	pr3 := mustPRState(t, "#3", "C", "feat-b", "feat-a", true, 0, nil) // cycle: feat-b -> feat-a

	done := make(chan struct{})
	go func() {
		defer close(done)
		report := domain.BuildPRConvergenceReport("main", []domain.PRState{pr1, pr2, pr3})
		// Chain should contain each PR at most once (3 unique PRs)
		if len(report.Chains) != 1 {
			t.Errorf("expected 1 chain, got %d", len(report.Chains))
			return
		}
		chain := report.Chains[0]
		if len(chain.PRs) > 3 {
			t.Errorf("expected at most 3 PRs in chain (no duplicates), got %d", len(chain.PRs))
		}
		// All 3 PRs should be in the chain (none orphaned)
		if len(report.OrphanedPRs) != 0 {
			t.Errorf("expected 0 orphaned PRs, got %d", len(report.OrphanedPRs))
		}
	}()

	select {
	case <-done:
		// OK — terminated
	case <-time.After(5 * time.Second):
		t.Fatal("buildChainBFS did not terminate — infinite loop detected (circular PR dependency)")
	}
}

func TestBuildPRChains_selfLoop_terminates(t *testing.T) {
	// given: PR whose head branch equals its own base's target (self-referential via adjacency)
	// main <- feat-a, feat-a <- feat-a (self-loop)
	pr1 := mustPRState(t, "#1", "A", "main", "feat-a", true, 0, nil)
	pr2 := mustPRState(t, "#2", "Self", "feat-a", "feat-a", true, 0, nil) // self-loop

	done := make(chan struct{})
	go func() {
		defer close(done)
		report := domain.BuildPRConvergenceReport("main", []domain.PRState{pr1, pr2})
		if len(report.Chains) != 1 {
			t.Errorf("expected 1 chain, got %d", len(report.Chains))
			return
		}
		// Self-loop PR should appear at most once
		chain := report.Chains[0]
		if len(chain.PRs) > 2 {
			t.Errorf("expected at most 2 PRs (no infinite duplication), got %d", len(chain.PRs))
		}
	}()

	select {
	case <-done:
		// OK
	case <-time.After(5 * time.Second):
		t.Fatal("buildChainBFS did not terminate — infinite loop detected (self-loop PR)")
	}
}

func TestBuildPRChains_longerCycle_terminates(t *testing.T) {
	// given: 4-node cycle: main <- a <- b <- c <- d <- a
	pr1 := mustPRState(t, "#1", "A", "main", "feat-a", true, 0, nil)
	pr2 := mustPRState(t, "#2", "B", "feat-a", "feat-b", true, 0, nil)
	pr3 := mustPRState(t, "#3", "C", "feat-b", "feat-c", true, 0, nil)
	pr4 := mustPRState(t, "#4", "D", "feat-c", "feat-d", true, 0, nil)
	pr5 := mustPRState(t, "#5", "E", "feat-d", "feat-a", true, 0, nil) // cycle back to feat-a

	done := make(chan struct{})
	go func() {
		defer close(done)
		report := domain.BuildPRConvergenceReport("main", []domain.PRState{pr1, pr2, pr3, pr4, pr5})
		if len(report.Chains) != 1 {
			t.Errorf("expected 1 chain, got %d", len(report.Chains))
			return
		}
		chain := report.Chains[0]
		if len(chain.PRs) > 5 {
			t.Errorf("expected at most 5 PRs, got %d", len(chain.PRs))
		}
		if len(report.OrphanedPRs) != 0 {
			t.Errorf("expected 0 orphaned, got %d", len(report.OrphanedPRs))
		}
	}()

	select {
	case <-done:
		// OK
	case <-time.After(5 * time.Second):
		t.Fatal("buildChainBFS did not terminate — infinite loop detected (longer cycle)")
	}
}

func TestBuildPRChains_cycleWithConflict(t *testing.T) {
	// given: cycle where one PR has conflict
	pr1 := mustPRState(t, "#1", "A", "main", "feat-a", true, 0, nil)
	pr2 := mustPRState(t, "#2", "B", "feat-a", "feat-b", false, 3, []string{"x.go"})
	pr3 := mustPRState(t, "#3", "C", "feat-b", "feat-a", true, 0, nil) // cycle

	done := make(chan struct{})
	go func() {
		defer close(done)
		report := domain.BuildPRConvergenceReport("main", []domain.PRState{pr1, pr2, pr3})
		if len(report.Chains) != 1 {
			t.Errorf("expected 1 chain, got %d", len(report.Chains))
			return
		}
		if !report.Chains[0].HasConflict {
			t.Error("expected chain to have conflict")
		}
	}()

	select {
	case <-done:
		// OK
	case <-time.After(5 * time.Second):
		t.Fatal("buildChainBFS did not terminate — infinite loop detected (cycle with conflict)")
	}
}

// --- PRState labels and headSHA tests ---

func TestPRState_HasLabel_exactMatch(t *testing.T) {
	// given
	ps, err := domain.NewPRState("#1", "title", "main", "feat", true, 0, nil,
		[]string{"amadeus:reviewed-abc12345", "bug"}, "abc12345def")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ps.HasLabel("bug") {
		t.Error("expected HasLabel(bug) = true")
	}
	if !ps.HasLabel("amadeus:reviewed-abc12345") {
		t.Error("expected HasLabel(amadeus:reviewed-abc12345) = true")
	}
	if ps.HasLabel("nonexistent") {
		t.Error("expected HasLabel(nonexistent) = false")
	}
}

func TestPRState_HasLabelPrefix(t *testing.T) {
	// given
	ps, err := domain.NewPRState("#1", "title", "main", "feat", true, 0, nil,
		[]string{"amadeus:reviewed-abc12345", "amadeus:reviewed-old1234"}, "abc12345def")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ps.HasLabelPrefix("amadeus:reviewed-") {
		t.Error("expected HasLabelPrefix(amadeus:reviewed-) = true")
	}
	if ps.HasLabelPrefix("sightjack:") {
		t.Error("expected HasLabelPrefix(sightjack:) = false")
	}
}

func TestPRState_HeadSHA(t *testing.T) {
	// given
	ps, err := domain.NewPRState("#1", "title", "main", "feat", true, 0, nil,
		nil, "abc12345def67890")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps.HeadSHA() != "abc12345def67890" {
		t.Errorf("HeadSHA = %q, want abc12345def67890", ps.HeadSHA())
	}
}

func TestPRState_HeadSHAShort(t *testing.T) {
	// given
	ps, err := domain.NewPRState("#1", "title", "main", "feat", true, 0, nil,
		nil, "abc12345def67890")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps.HeadSHAShort() != "abc12345" {
		t.Errorf("HeadSHAShort = %q, want abc12345", ps.HeadSHAShort())
	}
}

func TestPRState_ReviewLabel(t *testing.T) {
	// given: PR with head SHA and existing review label for OLD commit
	ps, err := domain.NewPRState("#1", "title", "main", "feat", true, 0, nil,
		[]string{"amadeus:reviewed-old12345"}, "newsha123abcdef")

	// then: not reviewed for current commit
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	currentLabel := "amadeus:reviewed-" + ps.HeadSHAShort()
	if ps.HasLabel(currentLabel) {
		t.Error("expected not to have review label for current commit")
	}
	// but has old label
	if !ps.HasLabel("amadeus:reviewed-old12345") {
		t.Error("expected to have old review label")
	}
}

func TestPRState_EmptyLabels(t *testing.T) {
	// given
	ps, err := domain.NewPRState("#1", "title", "main", "feat", true, 0, nil, nil, "")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ps.Labels()) != 0 {
		t.Errorf("expected 0 labels, got %d", len(ps.Labels()))
	}
	if ps.HeadSHA() != "" {
		t.Errorf("expected empty headSHA, got %q", ps.HeadSHA())
	}
	if ps.HeadSHAShort() != "" {
		t.Errorf("expected empty headSHAShort, got %q", ps.HeadSHAShort())
	}
}

// mustPRState is a test helper that creates a PRState or fails the test.
func mustPRState(t *testing.T, number, title, baseBranch, headBranch string, mergeable bool, behindBy int, conflictFiles []string) domain.PRState {
	t.Helper()
	ps, err := domain.NewPRState(number, title, baseBranch, headBranch, mergeable, behindBy, conflictFiles, nil, "")
	if err != nil {
		t.Fatalf("mustPRState: %v", err)
	}
	return ps
}
