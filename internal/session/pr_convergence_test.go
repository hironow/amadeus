package session

// white-box-reason: tests unexported PR convergence pipeline internals (phase transitions, retry logic)

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// --- Test doubles ---

type mockPRReader struct {
	prs []domain.PRState
	err error
}

func (m *mockPRReader) ListOpenPRs(_ context.Context, _ string) ([]domain.PRState, error) {
	return m.prs, m.err
}

// mockEmitter records calls for assertion. Implements port.CheckEventEmitter.
type mockEmitter struct {
	dmailsGenerated          []domain.DMail
	prConvergenceCheckedData *domain.PRConvergenceCheckedData
	emitDMailErr             error
	emitPRConvergenceErr     error
}

func (m *mockEmitter) EmitInboxConsumed(_ domain.InboxConsumedData, _ time.Time) error { return nil }
func (m *mockEmitter) EmitForceFullNextSet(_, _ float64, _ time.Time) error            { return nil }
func (m *mockEmitter) EmitDMailGenerated(dmail domain.DMail, _ time.Time) error {
	if m.emitDMailErr != nil {
		return m.emitDMailErr
	}
	m.dmailsGenerated = append(m.dmailsGenerated, dmail)
	return nil
}
func (m *mockEmitter) EmitConvergenceDetected(_ domain.ConvergenceAlert, _ time.Time) error {
	return nil
}
func (m *mockEmitter) EmitDMailCommented(_, _ string, _ time.Time) error { return nil }
func (m *mockEmitter) EmitCheck(_ domain.CheckResult, _ time.Time) error { return nil }
func (m *mockEmitter) EmitRunStarted(_ domain.RunStartedData, _ time.Time) error {
	return nil
}
func (m *mockEmitter) EmitRunStopped(_ domain.RunStoppedData, _ time.Time) error {
	return nil
}
func (m *mockEmitter) EmitPRConvergenceChecked(data domain.PRConvergenceCheckedData, _ time.Time) error {
	if m.emitPRConvergenceErr != nil {
		return m.emitPRConvergenceErr
	}
	m.prConvergenceCheckedData = &data
	return nil
}

// mockStateReader for NextDMailName
type mockStateReader struct {
	seq int
}

func (m *mockStateReader) LoadLatest() (domain.CheckResult, error) {
	return domain.CheckResult{}, nil
}
func (m *mockStateReader) ScanInbox(_ context.Context) ([]domain.DMail, error) {
	return nil, nil
}
func (m *mockStateReader) NextDMailName(_ domain.DMailKind) (string, error) {
	m.seq++
	return fmt.Sprintf("test-dmail-%03d", m.seq), nil
}
func (m *mockStateReader) LoadAllDMails() ([]domain.DMail, error) {
	return nil, nil
}
func (m *mockStateReader) LoadConsumed() ([]domain.ConsumedRecord, error) {
	return nil, nil
}
func (m *mockStateReader) LoadSyncState() (domain.SyncState, error) {
	return domain.SyncState{}, nil
}

// --- Helper ---

func mustPRState(t *testing.T, number, title, base, head string, mergeable bool, behindBy int, conflicts []string) domain.PRState {
	t.Helper()
	pr, err := domain.NewPRState(number, title, base, head, mergeable, behindBy, conflicts)
	if err != nil {
		t.Fatalf("NewPRState: %v", err)
	}
	return pr
}

func newTestAmadeusForPR(prReader port.GitHubPRReader, emitter port.CheckEventEmitter, store port.StateReader) *Amadeus {
	return &Amadeus{
		PRReader: prReader,
		Emitter:  emitter,
		Store:    store,
		Logger:   &domain.NopLogger{},
	}
}

// --- Tests ---

func TestRunPreMergePipeline_noPRReader(t *testing.T) {
	// given: PRReader is nil
	a := newTestAmadeusForPR(nil, &mockEmitter{}, &mockStateReader{})

	// when
	dmails, err := a.runPreMergePipeline(context.Background(), "main")

	// then: returns nil, nil
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if dmails != nil {
		t.Fatalf("expected nil dmails, got: %v", dmails)
	}
}

func TestRunPreMergePipeline_noPRs(t *testing.T) {
	// given: PRReader returns empty slice
	reader := &mockPRReader{prs: []domain.PRState{}}
	emitter := &mockEmitter{}
	a := newTestAmadeusForPR(reader, emitter, &mockStateReader{})

	// when
	dmails, err := a.runPreMergePipeline(context.Background(), "main")

	// then: no D-Mails, no error, no convergence event emitted
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(dmails) != 0 {
		t.Fatalf("expected 0 dmails, got: %d", len(dmails))
	}
	if emitter.prConvergenceCheckedData != nil {
		t.Error("expected no pr_convergence.checked event for empty PRs")
	}
}

func TestRunPreMergePipeline_singleChain(t *testing.T) {
	// given: 3 PRs forming a chain rooted at "main"
	//   main <- #1 (head: feat-a) <- #2 (head: feat-b) <- #3 (head: feat-c)
	pr1 := mustPRState(t, "#1", "Feature A", "main", "feat-a", true, 2, nil)
	pr2 := mustPRState(t, "#2", "Feature B", "feat-a", "feat-b", true, 0, nil)
	pr3 := mustPRState(t, "#3", "Feature C", "feat-b", "feat-c", true, 0, nil)

	reader := &mockPRReader{prs: []domain.PRState{pr1, pr2, pr3}}
	emitter := &mockEmitter{}
	store := &mockStateReader{}
	a := newTestAmadeusForPR(reader, emitter, store)

	// when
	dmails, err := a.runPreMergePipeline(context.Background(), "main")

	// then: 1 D-Mail for the chain (3 PRs > 1 triggers needsAction)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(dmails) != 1 {
		t.Fatalf("expected 1 dmail, got: %d", len(dmails))
	}
	if dmails[0].Kind != domain.KindImplFeedback {
		t.Errorf("expected kind %q, got %q", domain.KindImplFeedback, dmails[0].Kind)
	}
	// Emitter should record dmail_generated and pr_convergence.checked
	if len(emitter.dmailsGenerated) != 1 {
		t.Errorf("expected 1 emitted dmail, got: %d", len(emitter.dmailsGenerated))
	}
	if emitter.prConvergenceCheckedData == nil {
		t.Fatal("expected pr_convergence.checked event")
	}
	if emitter.prConvergenceCheckedData.TotalOpenPRs != 3 {
		t.Errorf("expected TotalOpenPRs=3, got %d", emitter.prConvergenceCheckedData.TotalOpenPRs)
	}
	if emitter.prConvergenceCheckedData.Chains != 1 {
		t.Errorf("expected Chains=1, got %d", emitter.prConvergenceCheckedData.Chains)
	}
	if emitter.prConvergenceCheckedData.DMails != 1 {
		t.Errorf("expected DMails=1, got %d", emitter.prConvergenceCheckedData.DMails)
	}
}

func TestRunPreMergePipeline_withConflict(t *testing.T) {
	// given: chain with conflict
	pr1 := mustPRState(t, "#10", "Conflict PR", "main", "feat-x", false, 3, []string{"file.go"})

	reader := &mockPRReader{prs: []domain.PRState{pr1}}
	emitter := &mockEmitter{}
	store := &mockStateReader{}
	a := newTestAmadeusForPR(reader, emitter, store)

	// when
	dmails, err := a.runPreMergePipeline(context.Background(), "main")

	// then: D-Mail with severity=high
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(dmails) != 1 {
		t.Fatalf("expected 1 dmail, got: %d", len(dmails))
	}
	if dmails[0].Severity != domain.SeverityHigh {
		t.Errorf("expected severity %q, got %q", domain.SeverityHigh, dmails[0].Severity)
	}
	// pr_convergence.checked should report 1 conflict
	if emitter.prConvergenceCheckedData == nil {
		t.Fatal("expected pr_convergence.checked event")
	}
	if emitter.prConvergenceCheckedData.ConflictPRs != 1 {
		t.Errorf("expected ConflictPRs=1, got %d", emitter.prConvergenceCheckedData.ConflictPRs)
	}
}

func TestRunPreMergePipeline_manyPRs(t *testing.T) {
	// given: 50 PRs — 3 chains + many orphans, simulating a busy repo.
	// Chain 1: main <- feat-a <- feat-b
	// Chain 2: main <- feat-x (with conflict)
	// Chain 3: main <- feat-y <- feat-z <- feat-w
	// Remaining: 44 orphaned PRs (base branch doesn't match integration)
	var prs []domain.PRState

	// Chain 1 (2 PRs)
	prs = append(prs, mustPRState(t, "#1", "Feature A", "main", "feat-a", true, 1, nil))
	prs = append(prs, mustPRState(t, "#2", "Feature B", "feat-a", "feat-b", true, 0, nil))

	// Chain 2 (1 PR with conflict)
	prs = append(prs, mustPRState(t, "#3", "Feature X", "main", "feat-x", false, 5, []string{"api.go"}))

	// Chain 3 (3 PRs)
	prs = append(prs, mustPRState(t, "#4", "Feature Y", "main", "feat-y", true, 0, nil))
	prs = append(prs, mustPRState(t, "#5", "Feature Z", "feat-y", "feat-z", true, 0, nil))
	prs = append(prs, mustPRState(t, "#6", "Feature W", "feat-z", "feat-w", true, 0, nil))

	// 44 orphaned PRs (targeting "develop" not "main")
	for i := 7; i <= 50; i++ {
		prs = append(prs, mustPRState(t, fmt.Sprintf("#%d", i),
			fmt.Sprintf("Orphan %d", i), "develop",
			fmt.Sprintf("orphan-%d", i), true, 0, nil))
	}

	reader := &mockPRReader{prs: prs}
	emitter := &mockEmitter{}
	store := &mockStateReader{}
	a := newTestAmadeusForPR(reader, emitter, store)

	// when
	dmails, err := a.runPreMergePipeline(context.Background(), "main")

	// then
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// Chain 1 (2 PRs, needs action) + Chain 2 (conflict) + Chain 3 (3 PRs, needs action)
	// = 3 D-Mails
	if len(dmails) != 3 {
		t.Errorf("expected 3 dmails for 3 actionable chains, got %d", len(dmails))
	}

	// pr_convergence.checked event
	if emitter.prConvergenceCheckedData == nil {
		t.Fatal("expected pr_convergence.checked event")
	}
	data := emitter.prConvergenceCheckedData
	if data.TotalOpenPRs != 50 {
		t.Errorf("TotalOpenPRs = %d, want 50", data.TotalOpenPRs)
	}
	if data.ConflictPRs != 1 {
		t.Errorf("ConflictPRs = %d, want 1", data.ConflictPRs)
	}
	if data.DMails != 3 {
		t.Errorf("DMails = %d, want 3", data.DMails)
	}
}

func TestRunPreMergePipeline_singleMergeablePR(t *testing.T) {
	// given: 1 PR, mergeable, not behind → no action needed
	pr1 := mustPRState(t, "#5", "Clean PR", "main", "feat-clean", true, 0, nil)

	reader := &mockPRReader{prs: []domain.PRState{pr1}}
	emitter := &mockEmitter{}
	store := &mockStateReader{}
	a := newTestAmadeusForPR(reader, emitter, store)

	// when
	dmails, err := a.runPreMergePipeline(context.Background(), "main")

	// then: no D-Mails needed (single PR, mergeable, not behind)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(dmails) != 0 {
		t.Fatalf("expected 0 dmails, got: %d", len(dmails))
	}
	// pr_convergence.checked should still be emitted (1 chain exists)
	if emitter.prConvergenceCheckedData == nil {
		t.Fatal("expected pr_convergence.checked event")
	}
	if emitter.prConvergenceCheckedData.DMails != 0 {
		t.Errorf("expected DMails=0, got %d", emitter.prConvergenceCheckedData.DMails)
	}
}
