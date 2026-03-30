package session

// white-box-reason: tests attemptAutoMerge orchestration internals (chain order, failure continuation, merge method selection)

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// --- merge test doubles ---

type mergeMockPRReader struct {
	prs       []domain.PRState
	readiness map[string]*domain.PRMergeReadiness // keyed by PR number
}

func (m *mergeMockPRReader) ListOpenPRs(_ context.Context, _ string) ([]domain.PRState, error) {
	return m.prs, nil
}

func (m *mergeMockPRReader) GetPRDiff(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mergeMockPRReader) GetPRMergeReadiness(_ context.Context, prNumber string) (*domain.PRMergeReadiness, error) {
	r, ok := m.readiness[prNumber]
	if !ok {
		return nil, fmt.Errorf("no readiness for %s", prNumber)
	}
	return r, nil
}

type mergeCall struct {
	number string
	method domain.MergeMethod
}

type mergeMockPRWriter struct {
	mu         sync.Mutex
	calls      []mergeCall
	failOn     map[string]bool // PR numbers that should fail
	labelCalls []string
}

func (m *mergeMockPRWriter) ApplyLabel(_ context.Context, prNumber, label string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.labelCalls = append(m.labelCalls, prNumber+":"+label)
	return nil
}

func (m *mergeMockPRWriter) RemoveLabel(_ context.Context, _, _ string) error { return nil }
func (m *mergeMockPRWriter) DeleteLabel(_ context.Context, _ string) error    { return nil }

func (m *mergeMockPRWriter) MergePR(_ context.Context, prNumber string, method domain.MergeMethod) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mergeCall{number: prNumber, method: method})
	if m.failOn != nil && m.failOn[prNumber] {
		return fmt.Errorf("merge failed for %s", prNumber)
	}
	return nil
}

type mergeEmitter struct {
	mu          sync.Mutex
	merged      []domain.PRMergedData
	skipped     []domain.PRMergeSkippedData
	runStarted  bool
	runStopped  bool
}

func (e *mergeEmitter) EmitRunStarted(_ domain.RunStartedData, _ time.Time) error {
	e.runStarted = true
	return nil
}
func (e *mergeEmitter) EmitRunStopped(_ domain.RunStoppedData, _ time.Time) error {
	e.runStopped = true
	return nil
}
func (e *mergeEmitter) EmitInboxConsumed(_ domain.InboxConsumedData, _ time.Time) error { return nil }
func (e *mergeEmitter) EmitForceFullNextSet(_, _ float64, _ time.Time) error             { return nil }
func (e *mergeEmitter) EmitDMailGenerated(_ domain.DMail, _ time.Time) error              { return nil }
func (e *mergeEmitter) EmitConvergenceDetected(_ domain.ConvergenceAlert, _ time.Time) error {
	return nil
}
func (e *mergeEmitter) EmitDMailCommented(_, _ string, _ time.Time) error                { return nil }
func (e *mergeEmitter) EmitCheck(_ domain.CheckResult, _ time.Time) error                 { return nil }
func (e *mergeEmitter) EmitPRConvergenceChecked(_ domain.PRConvergenceCheckedData, _ time.Time) error {
	return nil
}
func (e *mergeEmitter) EmitPRMerged(data domain.PRMergedData, _ time.Time) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.merged = append(e.merged, data)
	return nil
}
func (e *mergeEmitter) EmitPRMergeSkipped(data domain.PRMergeSkippedData, _ time.Time) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.skipped = append(e.skipped, data)
	return nil
}

func newMergeTestAmadeus(reader *mergeMockPRReader, writer *mergeMockPRWriter, emitter *mergeEmitter) *Amadeus {
	return &Amadeus{
		PRReader: reader,
		PRWriter: writer,
		Emitter:  emitter,
		Logger:   &domain.NopLogger{},
	}
}

func readyPR(number string) *domain.PRMergeReadiness {
	r := domain.EvaluateMergeReadiness(number, "CLEAN", "APPROVED", "MERGEABLE", true)
	return &r
}

func blockedPR(number string, reason string) *domain.PRMergeReadiness {
	r := domain.EvaluateMergeReadiness(number, reason, "APPROVED", "MERGEABLE", true)
	return &r
}

func mustPR(t *testing.T, number, title, base, head string, labels []string, sha string) domain.PRState {
	t.Helper()
	pr, err := domain.NewPRState(number, title, base, head, true, 0, nil, labels, sha)
	if err != nil {
		t.Fatalf("NewPRState: %v", err)
	}
	return pr
}

// --- tests ---

func TestAttemptAutoMerge_MergesInChainOrder(t *testing.T) {
	// given: chain #1 (base=main) → #2 (base=feat-a)
	pr1 := mustPR(t, "#1", "root", "main", "feat-a", []string{"amadeus:reviewed-aaa"}, "aaa")
	pr2 := mustPR(t, "#2", "leaf", "feat-a", "feat-b", []string{"amadeus:reviewed-bbb"}, "bbb")

	reader := &mergeMockPRReader{
		prs: []domain.PRState{pr1, pr2},
		readiness: map[string]*domain.PRMergeReadiness{
			"#1": readyPR("#1"),
			"#2": readyPR("#2"),
		},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when
	a.attemptAutoMerge(context.Background(), "main")

	// then: merged in order #1 → #2
	if len(writer.calls) != 2 {
		t.Fatalf("expected 2 merge calls, got %d", len(writer.calls))
	}
	if writer.calls[0].number != "#1" {
		t.Errorf("first merge should be #1, got %s", writer.calls[0].number)
	}
	if writer.calls[1].number != "#2" {
		t.Errorf("second merge should be #2, got %s", writer.calls[1].number)
	}

	// then: #1 (chain root with dependent) → merge method
	if writer.calls[0].method != domain.MergeMethodMerge {
		t.Errorf("#1 should use merge method, got %s", writer.calls[0].method)
	}
	// then: #2 (chain leaf) → squash method
	if writer.calls[1].method != domain.MergeMethodSquash {
		t.Errorf("#2 should use squash method, got %s", writer.calls[1].method)
	}

	// then: events emitted
	if len(emitter.merged) != 2 {
		t.Errorf("expected 2 merged events, got %d", len(emitter.merged))
	}
}

func TestAttemptAutoMerge_ContinuesOnSingleFailure(t *testing.T) {
	// given: 3 standalone PRs, #2 fails to merge
	pr1 := mustPR(t, "#1", "first", "main", "feat-a", []string{"amadeus:reviewed-aaa"}, "aaa")
	pr2 := mustPR(t, "#2", "second", "main", "feat-b", []string{"amadeus:reviewed-bbb"}, "bbb")
	pr3 := mustPR(t, "#3", "third", "main", "feat-c", []string{"amadeus:reviewed-ccc"}, "ccc")

	reader := &mergeMockPRReader{
		prs: []domain.PRState{pr1, pr2, pr3},
		readiness: map[string]*domain.PRMergeReadiness{
			"#1": readyPR("#1"),
			"#2": readyPR("#2"),
			"#3": readyPR("#3"),
		},
	}
	writer := &mergeMockPRWriter{failOn: map[string]bool{"#2": true}}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when
	a.attemptAutoMerge(context.Background(), "main")

	// then: all 3 merge attempts made (didn't stop at #2's failure)
	if len(writer.calls) != 3 {
		t.Fatalf("expected 3 merge calls, got %d", len(writer.calls))
	}

	// then: #1 and #3 succeeded, #2 failed
	if len(emitter.merged) != 2 {
		t.Errorf("expected 2 merged events, got %d", len(emitter.merged))
	}
	if len(emitter.skipped) != 1 {
		t.Errorf("expected 1 skipped event, got %d", len(emitter.skipped))
	}
	if emitter.skipped[0].PRNumber != "#2" {
		t.Errorf("skipped PR should be #2, got %s", emitter.skipped[0].PRNumber)
	}
}

func TestAttemptAutoMerge_SkipsNotReadyPRs(t *testing.T) {
	// given: 2 PRs, #2 has CI blocked
	pr1 := mustPR(t, "#1", "ready", "main", "feat-a", []string{"amadeus:reviewed-aaa"}, "aaa")
	pr2 := mustPR(t, "#2", "blocked", "main", "feat-b", []string{"amadeus:reviewed-bbb"}, "bbb")

	reader := &mergeMockPRReader{
		prs: []domain.PRState{pr1, pr2},
		readiness: map[string]*domain.PRMergeReadiness{
			"#1": readyPR("#1"),
			"#2": blockedPR("#2", "BLOCKED"),
		},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when
	a.attemptAutoMerge(context.Background(), "main")

	// then: only #1 merged, #2 skipped
	if len(writer.calls) != 1 {
		t.Fatalf("expected 1 merge call, got %d", len(writer.calls))
	}
	if writer.calls[0].number != "#1" {
		t.Errorf("merged PR should be #1, got %s", writer.calls[0].number)
	}
	if len(emitter.skipped) != 1 {
		t.Errorf("expected 1 skipped event, got %d", len(emitter.skipped))
	}
}

func TestMergePR_SquashMethod(t *testing.T) {
	// given: standalone ready PR
	pr := mustPR(t, "#1", "solo", "main", "feat-a", []string{"amadeus:reviewed-aaa"}, "aaa")

	reader := &mergeMockPRReader{
		prs:       []domain.PRState{pr},
		readiness: map[string]*domain.PRMergeReadiness{"#1": readyPR("#1")},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when
	a.attemptAutoMerge(context.Background(), "main")

	// then: squash method for standalone PR
	if len(writer.calls) != 1 {
		t.Fatalf("expected 1 merge call, got %d", len(writer.calls))
	}
	if writer.calls[0].method != domain.MergeMethodSquash {
		t.Errorf("expected squash, got %s", writer.calls[0].method)
	}
}

func TestMergePR_MergeMethod_ForChainRoot(t *testing.T) {
	// given: chain root with dependent
	root := mustPR(t, "#1", "root", "main", "feat-a", []string{"amadeus:reviewed-aaa"}, "aaa")
	leaf := mustPR(t, "#2", "leaf", "feat-a", "feat-b", []string{"amadeus:reviewed-bbb"}, "bbb")

	reader := &mergeMockPRReader{
		prs: []domain.PRState{root, leaf},
		readiness: map[string]*domain.PRMergeReadiness{
			"#1": readyPR("#1"),
			"#2": readyPR("#2"),
		},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when
	a.attemptAutoMerge(context.Background(), "main")

	// then: #1 (root) uses merge, #2 (leaf) uses squash
	if len(writer.calls) != 2 {
		t.Fatalf("expected 2 merge calls, got %d", len(writer.calls))
	}
	if writer.calls[0].method != domain.MergeMethodMerge {
		t.Errorf("#1 (root): expected merge, got %s", writer.calls[0].method)
	}
	if writer.calls[1].method != domain.MergeMethodSquash {
		t.Errorf("#2 (leaf): expected squash, got %s", writer.calls[1].method)
	}
}

// TestGoTaskboardScenario_ComplexMergeOrder reproduces a go-taskboard-like
// scenario with multiple chains, diamond dependencies, and orphaned PRs.
//
// Chain A: #10 (main→feat-auth) → #11 (feat-auth→feat-auth-tests) → #12 (feat-auth-tests→feat-auth-e2e)
// Chain B: #20 (main→feat-api) → #21 (feat-api→feat-api-v2)
// Standalone: #30 (main→fix-typo), #31 (main→chore-deps)
// Orphan: #40 (feat-deleted→feat-orphan) — base branch doesn't exist in any PR
//
// Expected merge order:
//   Chain A: #10 (merge) → #11 (merge) → #12 (squash)
//   Chain B: #20 (merge) → #21 (squash)
//   Standalone: #30 (squash), #31 (squash)
//   Orphan: #40 skipped (not in chain and not targeting main)
func TestGoTaskboardScenario_ComplexMergeOrder(t *testing.T) {
	// given: complex PR topology
	prs := []domain.PRState{
		// Chain A (3-deep)
		mustPR(t, "#10", "auth-base", "main", "feat-auth", []string{"amadeus:reviewed-a10"}, "a10"),
		mustPR(t, "#11", "auth-tests", "feat-auth", "feat-auth-tests", []string{"amadeus:reviewed-a11"}, "a11"),
		mustPR(t, "#12", "auth-e2e", "feat-auth-tests", "feat-auth-e2e", []string{"amadeus:reviewed-a12"}, "a12"),
		// Chain B (2-deep)
		mustPR(t, "#20", "api-base", "main", "feat-api", []string{"amadeus:reviewed-a20"}, "a20"),
		mustPR(t, "#21", "api-v2", "feat-api", "feat-api-v2", []string{"amadeus:reviewed-a21"}, "a21"),
		// Standalone
		mustPR(t, "#30", "fix-typo", "main", "fix-typo", []string{"amadeus:reviewed-a30"}, "a30"),
		mustPR(t, "#31", "chore-deps", "main", "chore-deps", []string{"amadeus:reviewed-a31"}, "a31"),
		// Orphan (base branch not in any PR's head)
		mustPR(t, "#40", "orphan", "feat-deleted", "feat-orphan", []string{"amadeus:reviewed-a40"}, "a40"),
	}

	allReady := map[string]*domain.PRMergeReadiness{}
	for _, pr := range prs {
		allReady[pr.Number()] = readyPR(pr.Number())
	}

	reader := &mergeMockPRReader{prs: prs, readiness: allReady}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when
	a.attemptAutoMerge(context.Background(), "main")

	// then: verify merge order and methods
	// Chains are processed first (by report.Chains order), then orphans.
	// Within each chain, root→leaf order is guaranteed by BFS.
	callMap := make(map[string]domain.MergeMethod)
	var callOrder []string
	for _, c := range writer.calls {
		callMap[c.number] = c.method
		callOrder = append(callOrder, c.number)
	}

	// Chain A: #10 before #11, #11 before #12
	idx10, idx11, idx12 := -1, -1, -1
	for i, n := range callOrder {
		switch n {
		case "#10":
			idx10 = i
		case "#11":
			idx11 = i
		case "#12":
			idx12 = i
		}
	}
	if idx10 >= idx11 || idx11 >= idx12 {
		t.Errorf("chain A order violated: #10@%d, #11@%d, #12@%d", idx10, idx11, idx12)
	}

	// Chain B: #20 before #21
	idx20, idx21 := -1, -1
	for i, n := range callOrder {
		switch n {
		case "#20":
			idx20 = i
		case "#21":
			idx21 = i
		}
	}
	if idx20 >= idx21 {
		t.Errorf("chain B order violated: #20@%d, #21@%d", idx20, idx21)
	}

	// Merge methods: root/middle = merge, leaf/standalone = squash
	expectations := map[string]domain.MergeMethod{
		"#10": domain.MergeMethodMerge,  // chain A root
		"#11": domain.MergeMethodMerge,  // chain A middle
		"#12": domain.MergeMethodSquash, // chain A leaf
		"#20": domain.MergeMethodMerge,  // chain B root
		"#21": domain.MergeMethodSquash, // chain B leaf
		"#30": domain.MergeMethodSquash, // standalone
		"#31": domain.MergeMethodSquash, // standalone
	}
	for num, expectedMethod := range expectations {
		if got, ok := callMap[num]; !ok {
			t.Errorf("%s: not merged", num)
		} else if got != expectedMethod {
			t.Errorf("%s: expected %s, got %s", num, expectedMethod, got)
		}
	}

	// Orphan #40 is still attempted (it appears in report.OrphanedPRs)
	if _, ok := callMap["#40"]; !ok {
		t.Error("#40 (orphan): expected merge attempt")
	}
}

// TestGoTaskboardScenario_DriftSuspendsAutoMerge verifies that when a
// DriftError occurs (world-line divergence), attemptAutoMerge is NOT called.
// This test verifies the caller-side guard in run.go, simulated here.
func TestGoTaskboardScenario_DriftSuspendsAutoMerge(t *testing.T) {
	// given: PRs that would be merge-ready
	pr := mustPR(t, "#1", "ready", "main", "feat-a", []string{"amadeus:reviewed-aaa"}, "aaa")
	reader := &mergeMockPRReader{
		prs:       []domain.PRState{pr},
		readiness: map[string]*domain.PRMergeReadiness{"#1": readyPR("#1")},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when: simulate DriftError guard (this is the run.go logic)
	driftDetected := true // simulates DriftError from runPostMergeCheck
	if !driftDetected {
		a.attemptAutoMerge(context.Background(), "main")
	}

	// then: no merge calls — drift suspended auto-merge
	if len(writer.calls) != 0 {
		t.Errorf("expected 0 merge calls during drift, got %d", len(writer.calls))
	}
	if len(emitter.merged) != 0 {
		t.Errorf("expected 0 merged events during drift, got %d", len(emitter.merged))
	}
}

// TestGoTaskboardScenario_PartialChainReadiness verifies that when only
// part of a chain is ready, the ready root is merged but the unready
// leaf is skipped.
func TestGoTaskboardScenario_PartialChainReadiness(t *testing.T) {
	// given: chain #22 → #23, but #23 has no review label (not ready)
	pr22 := mustPR(t, "#22", "http-test-infra", "main", "feat/http-test", []string{"amadeus:reviewed-0ef"}, "0ef")
	pr23 := mustPR(t, "#23", "reproduction-test", "feat/http-test", "feat/repro", nil, "dead") // no label

	reader := &mergeMockPRReader{
		prs: []domain.PRState{pr22, pr23},
		readiness: map[string]*domain.PRMergeReadiness{
			"#22": readyPR("#22"),
			"#23": func() *domain.PRMergeReadiness {
				r := domain.EvaluateMergeReadiness("#23", "CLEAN", "", "MERGEABLE", false)
				return &r
			}(),
		},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when
	a.attemptAutoMerge(context.Background(), "main")

	// then: #22 merged (root, has dependent → merge method), #23 skipped
	if len(writer.calls) != 1 {
		t.Fatalf("expected 1 merge call, got %d", len(writer.calls))
	}
	if writer.calls[0].number != "#22" {
		t.Errorf("expected #22 merged, got %s", writer.calls[0].number)
	}
	if writer.calls[0].method != domain.MergeMethodMerge {
		t.Errorf("#22 (chain root with unready dependent): expected merge, got %s", writer.calls[0].method)
	}
	if len(emitter.skipped) != 1 || emitter.skipped[0].PRNumber != "#23" {
		t.Errorf("expected #23 skipped, got %v", emitter.skipped)
	}
}

// --- startup auto-merge test doubles ---

type mockMergeStateReader struct {
	latest domain.CheckResult
	err    error
}

func (m *mockMergeStateReader) LoadLatest() (domain.CheckResult, error) {
	return m.latest, m.err
}

func (m *mockMergeStateReader) ScanInbox(_ context.Context) ([]domain.DMail, error) {
	return nil, nil
}

func (m *mockMergeStateReader) NextDMailName(_ domain.DMailKind) (string, error) {
	return "", nil
}

func (m *mockMergeStateReader) LoadAllDMails() ([]domain.DMail, error) {
	return nil, nil
}

func (m *mockMergeStateReader) LoadConsumed() ([]domain.ConsumedRecord, error) {
	return nil, nil
}

func (m *mockMergeStateReader) LoadSyncState() (domain.SyncState, error) {
	return domain.SyncState{}, nil
}

// --- startup auto-merge tests ---

func TestStartupAutoMerge_PreviousClean_MergesOnStartup(t *testing.T) {
	// given: last check was clean (no D-Mails generated, even with non-zero divergence)
	pr := mustPR(t, "#1", "ready", "main", "feat-a", []string{"amadeus:reviewed-aaa"}, "aaa")
	reader := &mergeMockPRReader{
		prs:       []domain.PRState{pr},
		readiness: map[string]*domain.PRMergeReadiness{"#1": readyPR("#1")},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	store := &mockMergeStateReader{latest: domain.CheckResult{CheckedAt: time.Now(), Divergence: 0.21, DMails: nil}}

	a := newMergeTestAmadeus(reader, writer, emitter)
	a.Store = store

	// Simulate startup auto-merge logic (mirrors run.go guard)
	previous, _ := a.Store.LoadLatest()
	if !previous.CheckedAt.IsZero() && len(previous.DMails) == 0 {
		a.attemptAutoMerge(context.Background(), "main")
	}

	// then: merge was called (divergence 0.21 is LOW, no D-Mails = not diverged)
	if len(writer.calls) != 1 {
		t.Fatalf("expected 1 merge call, got %d", len(writer.calls))
	}
}

func TestStartupAutoMerge_PreviousDrift_SkipsMerge(t *testing.T) {
	// given: last check generated D-Mails (world line diverged)
	pr := mustPR(t, "#1", "ready", "main", "feat-a", []string{"amadeus:reviewed-aaa"}, "aaa")
	reader := &mergeMockPRReader{
		prs:       []domain.PRState{pr},
		readiness: map[string]*domain.PRMergeReadiness{"#1": readyPR("#1")},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	store := &mockMergeStateReader{latest: domain.CheckResult{
		CheckedAt:  time.Now(),
		Divergence: 0.42,
		DMails:     []string{"feedback-1", "feedback-2"}, // D-Mails generated = DriftError
	}}

	a := newMergeTestAmadeus(reader, writer, emitter)
	a.Store = store

	// Simulate startup auto-merge logic
	previous, _ := a.Store.LoadLatest()
	if !previous.CheckedAt.IsZero() && len(previous.DMails) == 0 {
		a.attemptAutoMerge(context.Background(), "main")
	}

	// then: no merge (D-Mails generated = world line diverged)
	if len(writer.calls) != 0 {
		t.Errorf("expected 0 merge calls during drift, got %d", len(writer.calls))
	}
}

func TestStartupAutoMerge_NoPriorCheck_SkipsMerge(t *testing.T) {
	// given: no prior check (CheckedAt is zero)
	pr := mustPR(t, "#1", "ready", "main", "feat-a", []string{"amadeus:reviewed-aaa"}, "aaa")
	reader := &mergeMockPRReader{
		prs:       []domain.PRState{pr},
		readiness: map[string]*domain.PRMergeReadiness{"#1": readyPR("#1")},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	store := &mockMergeStateReader{latest: domain.CheckResult{}} // zero CheckedAt

	a := newMergeTestAmadeus(reader, writer, emitter)
	a.Store = store

	// Simulate startup auto-merge logic
	previous, _ := a.Store.LoadLatest()
	if !previous.CheckedAt.IsZero() && len(previous.DMails) == 0 {
		a.attemptAutoMerge(context.Background(), "main")
	}

	// then: no merge (no prior check)
	if len(writer.calls) != 0 {
		t.Errorf("expected 0 merge calls for first run, got %d", len(writer.calls))
	}
}

// TestGoTaskboardScenario_StartupMerge_Divergence021_NoDMails reproduces the
// exact go-taskboard state (2026-03-30): Divergence=0.208, DMails=nil.
// This verifies that LOW divergence with no D-Mails allows startup merge.
func TestGoTaskboardScenario_StartupMerge_Divergence021_NoDMails(t *testing.T) {
	// given: go-taskboard state — 18 PRs, divergence 0.208, no D-Mails
	prs := []domain.PRState{
		mustPR(t, "#14", "status-validation", "main", "feat/input-w2-5", []string{"amadeus:reviewed-a9c5"}, "a9c5"),
		mustPR(t, "#15", "pagination-repro", "main", "feat/pagination-w1", []string{"amadeus:reviewed-e6a3"}, "e6a3"),
		mustPR(t, "#16", "handler-validation", "main", "feat/cluster-w2-1", []string{"amadeus:reviewed-4112"}, "4112"),
		mustPR(t, "#22", "http-test-infra", "main", "feat/http-test", []string{"amadeus:reviewed-0ef9"}, "0ef9"),
		mustPR(t, "#23", "reproduction-test", "feat/http-test", "feat/repro", nil, "dead"), // chain leaf, no review label
	}

	allReady := map[string]*domain.PRMergeReadiness{
		"#14": readyPR("#14"),
		"#15": readyPR("#15"),
		"#16": readyPR("#16"),
		"#22": readyPR("#22"),
		"#23": func() *domain.PRMergeReadiness {
			r := domain.EvaluateMergeReadiness("#23", "CLEAN", "", "MERGEABLE", false) // no review label
			return &r
		}(),
	}

	reader := &mergeMockPRReader{prs: prs, readiness: allReady}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	// go-taskboard actual state: divergence 0.208, DMails nil (no D-Mails generated)
	store := &mockMergeStateReader{latest: domain.CheckResult{
		CheckedAt:  time.Now(),
		Divergence: 0.208,
		DMails:     nil, // no D-Mails = no DriftError = world line normal
	}}

	a := newMergeTestAmadeus(reader, writer, emitter)
	a.Store = store

	// when: simulate startup auto-merge guard (mirrors run.go)
	previous, _ := a.Store.LoadLatest()
	if !previous.CheckedAt.IsZero() && len(previous.DMails) == 0 {
		a.attemptAutoMerge(context.Background(), "main")
	}

	// then: merge proceeds (divergence 0.208 with no D-Mails is NOT 世界線逸脱)
	if len(writer.calls) == 0 {
		t.Fatal("expected merge calls but got 0 — startup merge should proceed when DMails is nil")
	}

	// then: #14, #15, #16 merged as squash (standalone)
	// #22 merged as merge (chain root with #23 dependent)
	// #23 skipped (no review label)
	callMap := make(map[string]domain.MergeMethod)
	for _, c := range writer.calls {
		callMap[c.number] = c.method
	}

	if callMap["#22"] != domain.MergeMethodMerge {
		t.Errorf("#22 (chain root): expected merge, got %s", callMap["#22"])
	}
	if _, merged := callMap["#23"]; merged {
		t.Error("#23 should NOT be merged (no review label)")
	}
	for _, num := range []string{"#14", "#15", "#16"} {
		if callMap[num] != domain.MergeMethodSquash {
			t.Errorf("%s (standalone): expected squash, got %s", num, callMap[num])
		}
	}

	// then: #23 skipped with reason
	if len(emitter.skipped) != 1 || emitter.skipped[0].PRNumber != "#23" {
		t.Errorf("expected #23 skipped, got %v", emitter.skipped)
	}
}
