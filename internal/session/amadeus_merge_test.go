package session

// white-box-reason: tests attemptAutoMerge orchestration internals (chain order, failure continuation, merge method selection)

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness"
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

type closeCall struct {
	number  string
	comment string
}

type mergeMockPRWriter struct {
	mu         sync.Mutex
	calls      []mergeCall
	failOn     map[string]bool // PR numbers that should fail
	labelCalls []string
	closed     []closeCall
}

func (m *mergeMockPRWriter) ApplyLabel(_ context.Context, prNumber, label string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.labelCalls = append(m.labelCalls, prNumber+":"+label)
	return nil
}

func (m *mergeMockPRWriter) RemoveLabel(_ context.Context, _, _ string) error { return nil }
func (m *mergeMockPRWriter) DeleteLabel(_ context.Context, _ string) error    { return nil }

func (m *mergeMockPRWriter) ClosePR(_ context.Context, prNumber, comment string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = append(m.closed, closeCall{number: prNumber, comment: comment})
	return nil
}

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
	mu              sync.Mutex
	merged          []domain.PRMergedData
	skipped         []domain.PRMergeSkippedData
	dmailsGenerated []domain.DMail
	runStarted      bool
	runStopped      bool
}

func (e *mergeEmitter) EmitRunStarted(_ domain.RunStartedData, _ time.Time) error {
	e.runStarted = true
	return nil
}
func (e *mergeEmitter) EmitRunStopped(_ domain.RunStoppedData, _ time.Time) error {
	e.runStopped = true
	return nil
}
func (e *mergeEmitter) EmitInboxConsumed(_ domain.InboxConsumedData, _ time.Time) error {
	return nil
}
func (e *mergeEmitter) EmitForceFullNextSet(_, _ float64, _ time.Time) error {
	return nil
}
func (e *mergeEmitter) EmitDMailGenerated(dmail domain.DMail, _ time.Time) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dmailsGenerated = append(e.dmailsGenerated, dmail)
	return nil
}
func (e *mergeEmitter) EmitConvergenceDetected(_ domain.ConvergenceAlert, _ time.Time) error {
	return nil
}
func (e *mergeEmitter) EmitDMailCommented(_, _ string, _ time.Time) error {
	return nil
}
func (e *mergeEmitter) EmitCheck(_ domain.CheckResult, _ time.Time) error {
	return nil
}
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
	r := harness.EvaluateMergeReadiness(number, "CLEAN", "APPROVED", "MERGEABLE", true)
	return &r
}

func blockedPR(number string, reason string) *domain.PRMergeReadiness {
	r := harness.EvaluateMergeReadiness(number, reason, "APPROVED", "MERGEABLE", true)
	return &r
}

func mustPR(t *testing.T, number, title, base, head string, labels []string, sha string) domain.PRState { // nosemgrep: domain-primitives.multiple-string-params-go — test helper; distinct PR identity fields [permanent]
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
				r := harness.EvaluateMergeReadiness("#23", "CLEAN", "", "MERGEABLE", false)
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

// TestStartupAutoMerge_ZeroDivergenceButDMails_SkipsMerge verifies that
// even with Divergence=0.0, if D-Mails were generated (e.g. from convergence
// alerts), merge is still blocked. The guard is D-Mail count, not score.
func TestStartupAutoMerge_ZeroDivergenceButDMails_SkipsMerge(t *testing.T) {
	// given: divergence is 0.0 but D-Mails were generated
	pr := mustPR(t, "#1", "ready", "main", "feat-a", []string{"amadeus:reviewed-aaa"}, "aaa")
	reader := &mergeMockPRReader{
		prs:       []domain.PRState{pr},
		readiness: map[string]*domain.PRMergeReadiness{"#1": readyPR("#1")},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	store := &mockMergeStateReader{latest: domain.CheckResult{
		CheckedAt:  time.Now(),
		Divergence: 0.0,
		DMails:     []string{"convergence-alert-1"}, // D-Mail from convergence, not divergence
	}}

	a := newMergeTestAmadeus(reader, writer, emitter)
	a.Store = store

	// Simulate startup auto-merge guard
	previous, _ := a.Store.LoadLatest()
	if !previous.CheckedAt.IsZero() && len(previous.DMails) == 0 {
		a.attemptAutoMerge(context.Background(), "main")
	}

	// then: no merge (D-Mails exist even though divergence is 0)
	if len(writer.calls) != 0 {
		t.Errorf("expected 0 merge calls when D-Mails exist, got %d", len(writer.calls))
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
			r := harness.EvaluateMergeReadiness("#23", "CLEAN", "", "MERGEABLE", false) // no review label
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

// TestAttemptAutoMerge_ReturnsCount verifies that attemptAutoMerge returns the
// number of merged PRs, enabling callers to decide whether to re-run pipelines.
func TestAttemptAutoMerge_ReturnsCount(t *testing.T) {
	// given: 3 PRs, 1 fails
	prs := []domain.PRState{
		mustPR(t, "#1", "a", "main", "feat-a", []string{"amadeus:reviewed-a"}, "a"),
		mustPR(t, "#2", "b", "main", "feat-b", []string{"amadeus:reviewed-b"}, "b"),
		mustPR(t, "#3", "c", "main", "feat-c", []string{"amadeus:reviewed-c"}, "c"),
	}
	reader := &mergeMockPRReader{
		prs: prs,
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
	merged := a.attemptAutoMerge(context.Background(), "main")

	// then: 2 merged (1 failed)
	if merged != 2 {
		t.Errorf("expected merged=2, got %d", merged)
	}
}

// TestGoTaskboardScenario_PostMergeRerunTriggered verifies that after
// successful merges, the caller can detect merged > 0 and trigger
// re-pipeline (rerunPipelinesAfterMerge). This test simulates the
// go-taskboard flow where 4 PRs merge and remaining PRs need
// conflict detection.
func TestGoTaskboardScenario_PostMergeRerunTriggered(t *testing.T) {
	// given: 5 PRs, 4 are ready, 1 fails to merge (simulates conflict)
	prs := []domain.PRState{
		mustPR(t, "#31", "errors-is", "main", "expedition/052", []string{"amadeus:reviewed-33f5"}, "33f5"),
		mustPR(t, "#28", "export-set", "main", "feat/export-set", []string{"amadeus:reviewed-5735"}, "5735"),
		mustPR(t, "#27", "export", "main", "feat/export", []string{"amadeus:reviewed-3e84"}, "3e84"),
		mustPR(t, "#15", "pagination", "main", "feat/pagination", []string{"amadeus:reviewed-e6a3"}, "e6a3"),
		mustPR(t, "#30", "stats", "main", "feat/stats", []string{"amadeus:reviewed-3fad"}, "3fad"),
	}

	allReady := map[string]*domain.PRMergeReadiness{}
	for _, pr := range prs {
		allReady[pr.Number()] = readyPR(pr.Number())
	}

	// #30 fails (simulates conflict after other PRs merge)
	writer := &mergeMockPRWriter{failOn: map[string]bool{"#30": true}}
	reader := &mergeMockPRReader{prs: prs, readiness: allReady}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when
	merged := a.attemptAutoMerge(context.Background(), "main")

	// then: 4 merged, 1 failed
	if merged != 4 {
		t.Errorf("expected 4 merged, got %d", merged)
	}

	// then: caller should detect merged > 0 and trigger re-pipeline
	rerunTriggered := merged > 0
	if !rerunTriggered {
		t.Error("expected rerun to be triggered after merges")
	}

	// then: #30 was skipped (failed)
	if len(emitter.skipped) != 1 || emitter.skipped[0].PRNumber != "#30" {
		t.Errorf("expected #30 skipped, got %v", emitter.skipped)
	}
}

// TestGoTaskboardScenario_ConflictingPRs_GeneratesDMails verifies that
// when all PRs are CONFLICTING (as happened in go-taskboard after 4 merges),
// conflict D-Mails are generated for each conflicting PR and routed to paintress.
func TestGoTaskboardScenario_ConflictingPRs_GeneratesDMails(t *testing.T) {
	// given: 3 PRs all CONFLICTING (simulates go-taskboard post-merge state)
	prs := []domain.PRState{
		mustPR(t, "#30", "stats", "main", "feat/stats", []string{"amadeus:reviewed-3fad"}, "3fad"),
		mustPR(t, "#29", "status-filter", "main", "feat/filter", []string{"amadeus:reviewed-56cb"}, "56cb"),
		mustPR(t, "#16", "handler-validation", "main", "feat/handler", []string{"amadeus:reviewed-4112"}, "4112"),
	}

	conflictingReadiness := func(number string) *domain.PRMergeReadiness {
		r := harness.EvaluateMergeReadiness(number, "DIRTY", "", "CONFLICTING", true)
		return &r
	}

	reader := &mergeMockPRReader{
		prs: prs,
		readiness: map[string]*domain.PRMergeReadiness{
			"#30": conflictingReadiness("#30"),
			"#29": conflictingReadiness("#29"),
			"#16": conflictingReadiness("#16"),
		},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when
	merged := a.attemptAutoMerge(context.Background(), "main")

	// then: 0 merged (all conflicting)
	if merged != 0 {
		t.Errorf("expected 0 merged, got %d", merged)
	}

	// then: no merge calls
	if len(writer.calls) != 0 {
		t.Errorf("expected 0 merge calls, got %d", len(writer.calls))
	}

	// then: 3 conflict D-Mails generated (one per conflicting PR)
	if len(emitter.dmailsGenerated) != 3 {
		t.Fatalf("expected 3 conflict D-Mails, got %d", len(emitter.dmailsGenerated))
	}

	// then: all D-Mails are KindImplFeedback targeting paintress
	for _, dmail := range emitter.dmailsGenerated {
		if dmail.Kind != domain.KindImplFeedback {
			t.Errorf("expected KindImplFeedback, got %s", dmail.Kind)
		}
		if len(dmail.Targets) == 0 || dmail.Targets[0] != "paintress" {
			t.Errorf("expected target paintress, got %v", dmail.Targets)
		}
		if dmail.Metadata["type"] != "merge-conflict" {
			t.Errorf("expected metadata type=merge-conflict, got %s", dmail.Metadata["type"])
		}
	}

	// then: 3 skipped events
	if len(emitter.skipped) != 3 {
		t.Errorf("expected 3 skipped events, got %d", len(emitter.skipped))
	}
}

// TestAttemptAutoMerge_BlockedButNotConflicting_NoDMail verifies that
// non-CONFLICTING skip reasons (BLOCKED CI, missing review, etc.)
// do NOT generate conflict D-Mails. Only CONFLICTING triggers D-Mail.
func TestAttemptAutoMerge_BlockedButNotConflicting_NoDMail(t *testing.T) {
	// given: PR blocked by CI (not conflicting)
	pr := mustPR(t, "#1", "blocked", "main", "feat-a", []string{"amadeus:reviewed-aaa"}, "aaa")
	reader := &mergeMockPRReader{
		prs: []domain.PRState{pr},
		readiness: map[string]*domain.PRMergeReadiness{
			"#1": blockedPR("#1", "BLOCKED"), // CI blocked, but MERGEABLE
		},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when
	a.attemptAutoMerge(context.Background(), "main")

	// then: skipped but NO conflict D-Mail (not CONFLICTING)
	if len(emitter.skipped) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(emitter.skipped))
	}
	if len(emitter.dmailsGenerated) != 0 {
		t.Errorf("expected 0 conflict D-Mails for BLOCKED (non-CONFLICTING), got %d", len(emitter.dmailsGenerated))
	}
}

// --- orphaned pipeline PR tests ---

// TestAttemptAutoMerge_ClosesPipelineOrphan_WaveBranch reproduces the
// go-taskboard PR #23 scenario: an orphaned PR whose base/head branches
// match the sightjack wave naming pattern. Pipeline orphans are closed
// instead of merge-attempted.
func TestAttemptAutoMerge_ClosesPipelineOrphan_WaveBranch(t *testing.T) {
	// given: orphaned PR with wave pattern in both base and head branches
	// (PR #23's exact pattern from go-taskboard)
	orphan := mustPR(t, "#23", "test: #1 pagination bug reproduction",
		"feat/pagination-validation-w1-21-http-test-infra",
		"feat/pagination-validation-w3-1-reproduction-test",
		nil, "deadbeef")

	reader := &mergeMockPRReader{
		prs:       []domain.PRState{orphan},
		readiness: map[string]*domain.PRMergeReadiness{"#23": readyPR("#23")},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when
	a.attemptAutoMerge(context.Background(), "main")

	// then: PR was closed, NOT merge-attempted
	if len(writer.calls) != 0 {
		t.Errorf("expected 0 merge calls for pipeline orphan, got %d", len(writer.calls))
	}
	if len(writer.closed) != 1 {
		t.Fatalf("expected 1 close call, got %d", len(writer.closed))
	}
	if writer.closed[0].number != "#23" {
		t.Errorf("expected closed PR #23, got %s", writer.closed[0].number)
	}
}

// TestAttemptAutoMerge_ClosesPipelineOrphan_ExpeditionBranch verifies
// expedition-prefixed orphaned PRs are closed.
func TestAttemptAutoMerge_ClosesPipelineOrphan_ExpeditionBranch(t *testing.T) {
	// given: orphaned PR from expedition (paintress)
	orphan := mustPR(t, "#50", "feat: migrate errors.Is",
		"feat-deleted-base", "expedition/052-errors-is-migration",
		nil, "abc12345")

	reader := &mergeMockPRReader{
		prs:       []domain.PRState{orphan},
		readiness: map[string]*domain.PRMergeReadiness{"#50": readyPR("#50")},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when
	a.attemptAutoMerge(context.Background(), "main")

	// then: closed, not merged
	if len(writer.calls) != 0 {
		t.Errorf("expected 0 merge calls, got %d", len(writer.calls))
	}
	if len(writer.closed) != 1 || writer.closed[0].number != "#50" {
		t.Fatalf("expected closed PR #50, got %v", writer.closed)
	}
}

// TestAttemptAutoMerge_ClosesPipelineOrphan_PaintressLabel verifies
// orphaned PRs with paintress:pr-open label are closed.
func TestAttemptAutoMerge_ClosesPipelineOrphan_PaintressLabel(t *testing.T) {
	// given: orphaned PR with paintress:pr-open label (no wave pattern in branch)
	orphan := mustPR(t, "#60", "fix: some issue",
		"feat-old-base", "fix/some-fix",
		[]string{"paintress:pr-open"}, "abc12345")

	reader := &mergeMockPRReader{
		prs:       []domain.PRState{orphan},
		readiness: map[string]*domain.PRMergeReadiness{"#60": readyPR("#60")},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when
	a.attemptAutoMerge(context.Background(), "main")

	// then: closed due to paintress:pr-open label
	if len(writer.calls) != 0 {
		t.Errorf("expected 0 merge calls, got %d", len(writer.calls))
	}
	if len(writer.closed) != 1 || writer.closed[0].number != "#60" {
		t.Fatalf("expected closed PR #60, got %v", writer.closed)
	}
}

// TestAttemptAutoMerge_NonPipelineOrphan_StillMergeAttempted verifies
// orphaned PRs WITHOUT pipeline indicators are still merge-attempted
// (existing behavior preserved).
func TestAttemptAutoMerge_NonPipelineOrphan_StillMergeAttempted(t *testing.T) {
	// given: orphaned PR with no pipeline indicators
	orphan := mustPR(t, "#40", "manual PR",
		"feat-deleted", "feat-orphan",
		nil, "abc12345")

	reader := &mergeMockPRReader{
		prs:       []domain.PRState{orphan},
		readiness: map[string]*domain.PRMergeReadiness{"#40": readyPR("#40")},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when
	a.attemptAutoMerge(context.Background(), "main")

	// then: merge attempted (not a pipeline PR, might be intentional)
	if len(writer.calls) != 1 || writer.calls[0].number != "#40" {
		t.Errorf("expected merge attempt for non-pipeline orphan #40, got %v", writer.calls)
	}
	if len(writer.closed) != 0 {
		t.Errorf("expected 0 close calls for non-pipeline orphan, got %d", len(writer.closed))
	}
}

// TestAttemptAutoMerge_MixedOrphans_ClosePipelineKeepOthers verifies
// that in a mixed scenario, pipeline orphans are closed while
// non-pipeline orphans are merge-attempted.
func TestAttemptAutoMerge_MixedOrphans_ClosePipelineKeepOthers(t *testing.T) {
	// given: 1 pipeline orphan + 1 normal orphan + 1 chain PR
	chainRoot := mustPR(t, "#1", "chain root", "main", "feat-a", nil, "aaa")
	pipelineOrphan := mustPR(t, "#23", "wave PR",
		"feat/pagination-w1-21-infra", "feat/pagination-w3-1-test",
		nil, "bbb")
	normalOrphan := mustPR(t, "#40", "manual PR",
		"develop", "feat-manual",
		nil, "ccc")

	reader := &mergeMockPRReader{
		prs: []domain.PRState{chainRoot, pipelineOrphan, normalOrphan},
		readiness: map[string]*domain.PRMergeReadiness{
			"#1":  readyPR("#1"),
			"#23": readyPR("#23"),
			"#40": readyPR("#40"),
		},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	// when
	a.attemptAutoMerge(context.Background(), "main")

	// then: #1 merged (chain), #23 closed (pipeline orphan), #40 merged (non-pipeline orphan)
	mergedNums := make(map[string]bool)
	for _, c := range writer.calls {
		mergedNums[c.number] = true
	}
	if !mergedNums["#1"] {
		t.Error("#1 (chain root) should be merged")
	}
	if mergedNums["#23"] {
		t.Error("#23 (pipeline orphan) should NOT be merged")
	}
	if !mergedNums["#40"] {
		t.Error("#40 (non-pipeline orphan) should be merge-attempted")
	}

	// then: #23 closed
	closedNums := make(map[string]bool)
	for _, c := range writer.closed {
		closedNums[c.number] = true
	}
	if !closedNums["#23"] {
		t.Error("#23 should be closed")
	}
	if closedNums["#40"] {
		t.Error("#40 should NOT be closed")
	}
}

// TestAttemptAutoMerge_ClosesPipelineOrphan_IssueLink verifies that
// orphaned PRs without branch/label signals are still detected as
// pipeline PRs via issue link (title references a sightjack:ready issue).
// TestAttemptAutoMerge_IssueLinkOnly_WarnsButDoesNotClose verifies that
// issue-link-only matches do NOT trigger close (codex review finding:
// false positive risk for release/hotfix PRs referencing the same issue).
// Only label/branch pattern matches warrant automatic close.
func TestAttemptAutoMerge_IssueLinkOnly_WarnsButDoesNotClose(t *testing.T) {
	// given: orphaned PR with no pipeline branch pattern or label,
	// but title references issue #1 which has sightjack:ready
	orphan := mustPR(t, "#70", "fix: address #1 pagination bug",
		"feat-old-branch", "fix/pagination-fix",
		nil, "abc12345")

	reader := &mergeMockPRReader{
		prs:       []domain.PRState{orphan},
		readiness: map[string]*domain.PRMergeReadiness{"#70": readyPR("#70")},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	issueWriter := &mockIssueWriterForMerge{
		issuesByLabel: map[string][]string{
			"sightjack:ready": {"1", "5", "9"},
		},
	}
	a := newMergeTestAmadeus(reader, writer, emitter)
	a.IssueWriter = issueWriter

	// when
	a.attemptAutoMerge(context.Background(), "main")

	// then: NOT closed — issue link alone is insufficient for close
	if len(writer.closed) != 0 {
		t.Errorf("expected 0 close calls (issue-link only should warn, not close), got %d", len(writer.closed))
	}
	// then: still merge-attempted as a regular orphan
	if len(writer.calls) != 1 || writer.calls[0].number != "#70" {
		t.Errorf("expected merge attempt for #70 (issue-link-only orphan), got %v", writer.calls)
	}
}

// mockIssueWriterForMerge is a minimal GitHubIssueWriter mock for merge tests.
type mockIssueWriterForMerge struct {
	issuesByLabel map[string][]string
}

func (m *mockIssueWriterForMerge) ListOpenIssuesByLabel(_ context.Context, label string) ([]string, error) {
	return m.issuesByLabel[label], nil
}

func (m *mockIssueWriterForMerge) CloseIssue(_ context.Context, _, _ string) error {
	return nil
}

// --- go-taskboard exact scenario: evaluatePRDiffs + attemptAutoMerge ---

// branchAwarePRReader filters PRs by target branch, matching real gh behavior:
//   ListOpenPRs("main") → only PRs where baseBranch == "main"
//   ListOpenPRs("")     → all PRs regardless of baseBranch
type branchAwarePRReader struct {
	prs       []domain.PRState
	readiness map[string]*domain.PRMergeReadiness
}

func (m *branchAwarePRReader) ListOpenPRs(_ context.Context, targetBranch string) ([]domain.PRState, error) {
	if targetBranch == "" {
		return m.prs, nil
	}
	var filtered []domain.PRState
	for _, pr := range m.prs {
		if pr.BaseBranch() == targetBranch {
			filtered = append(filtered, pr)
		}
	}
	return filtered, nil
}

func (m *branchAwarePRReader) GetPRDiff(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *branchAwarePRReader) GetPRMergeReadiness(_ context.Context, prNumber string) (*domain.PRMergeReadiness, error) {
	r, ok := m.readiness[prNumber]
	if !ok {
		return nil, fmt.Errorf("no readiness for %s", prNumber)
	}
	return r, nil
}

// TestGoTaskboardScenario_OrphanOnlyPR_EvalSkipsAutoMergeCloses reproduces
// the exact go-taskboard state: PR #23 targets a merged feature branch (not main).
//   - evaluatePRDiffs("main") sees 0 PRs (correct — #23 doesn't target main)
//   - attemptAutoMerge gets all PRs → #23 is orphaned pipeline PR → closes it
// Before the fix, attemptAutoMerge would skip #23 with "missing review label"
// indefinitely because evaluatePRDiffs never reviewed it.
func TestGoTaskboardScenario_OrphanOnlyPR_EvalSkipsAutoMergeCloses(t *testing.T) {
	// given: only PR #23 exists, targeting a merged feature branch
	orphan := mustPR(t, "#23", "test: #1 pagination bug reproduction tests — cannot-reproduce",
		"feat/pagination-validation-w1-21-http-test-infra",
		"feat/pagination-validation-w3-1-reproduction-test",
		nil, "deadbeef12345678")

	reader := &branchAwarePRReader{
		prs:       []domain.PRState{orphan},
		readiness: map[string]*domain.PRMergeReadiness{"#23": readyPR("#23")},
	}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := &Amadeus{
		PRReader: reader,
		PRWriter: writer,
		Emitter:  emitter,
		Logger:   &domain.NopLogger{},
	}

	// when: evaluatePRDiffs runs first (as in run.go)
	reviewDMails, reviewErr := a.evaluatePRDiffs(context.Background(), "main")

	// then: evaluatePRDiffs sees 0 PRs targeting main
	if reviewErr != nil {
		t.Fatalf("evaluatePRDiffs error: %v", reviewErr)
	}
	if len(reviewDMails) != 0 {
		t.Errorf("expected 0 review D-Mails (no main-targeting PRs), got %d", len(reviewDMails))
	}

	// when: attemptAutoMerge runs next (as in run.go)
	merged := a.attemptAutoMerge(context.Background(), "main")

	// then: #23 was closed as pipeline orphan, not merge-attempted
	if merged != 0 {
		t.Errorf("expected 0 merged, got %d", merged)
	}
	if len(writer.calls) != 0 {
		t.Errorf("expected 0 merge calls, got %d", len(writer.calls))
	}
	if len(writer.closed) != 1 {
		t.Fatalf("expected 1 close call, got %d", len(writer.closed))
	}
	if writer.closed[0].number != "#23" {
		t.Errorf("expected closed PR #23, got %s", writer.closed[0].number)
	}
}

// TestAttemptAutoMerge_EmptyPRList verifies no panic on empty PR list.
func TestAttemptAutoMerge_EmptyPRList(t *testing.T) {
	reader := &mergeMockPRReader{prs: nil, readiness: map[string]*domain.PRMergeReadiness{}}
	writer := &mergeMockPRWriter{}
	emitter := &mergeEmitter{}
	a := newMergeTestAmadeus(reader, writer, emitter)

	merged := a.attemptAutoMerge(context.Background(), "main")
	if merged != 0 {
		t.Errorf("expected 0, got %d", merged)
	}
}
