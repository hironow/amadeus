package usecase

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// --- Test doubles ---

type mockPRReader struct {
	prs []domain.PRState
	err error
}

func (m *mockPRReader) ListOpenPRs(_ context.Context, _ string) ([]domain.PRState, error) {
	return m.prs, m.err
}

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

type mockStateReader struct {
	seq int
}

func (m *mockStateReader) LoadLatest() (domain.CheckResult, error)                  { return domain.CheckResult{}, nil }
func (m *mockStateReader) ScanInbox(_ context.Context) ([]domain.DMail, error)      { return nil, nil }
func (m *mockStateReader) NextDMailName(_ domain.DMailKind) (string, error)          { m.seq++; return fmt.Sprintf("test-dmail-%03d", m.seq), nil }
func (m *mockStateReader) LoadAllDMails() ([]domain.DMail, error)                   { return nil, nil }
func (m *mockStateReader) LoadConsumed() ([]domain.ConsumedRecord, error)            { return nil, nil }
func (m *mockStateReader) LoadSyncState() (domain.SyncState, error)                  { return domain.SyncState{}, nil }

// --- Helper ---

func mustPRState(t *testing.T, number, title, base, head string, mergeable bool, behindBy int, conflicts []string) domain.PRState {
	t.Helper()
	pr, err := domain.NewPRState(number, title, base, head, mergeable, behindBy, conflicts)
	if err != nil {
		t.Fatalf("NewPRState: %v", err)
	}
	return pr
}

// --- Tests ---

func TestRunPreMergePipeline_noPRReader(t *testing.T) {
	// given: PRReader is nil
	emitter := &mockEmitter{}
	store := &mockStateReader{}

	// when
	dmails, err := runPreMergePipeline(context.Background(), "main", nil, store, emitter, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if dmails != nil {
		t.Fatalf("expected nil dmails, got: %v", dmails)
	}
}

func TestRunPreMergePipeline_noPRs(t *testing.T) {
	// given
	reader := &mockPRReader{prs: []domain.PRState{}}
	emitter := &mockEmitter{}
	store := &mockStateReader{}

	// when
	dmails, err := runPreMergePipeline(context.Background(), "main", reader, store, emitter, &domain.NopLogger{})

	// then
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
	// given: 3 PRs forming a chain
	pr1 := mustPRState(t, "#1", "Feature A", "main", "feat-a", true, 2, nil)
	pr2 := mustPRState(t, "#2", "Feature B", "feat-a", "feat-b", true, 0, nil)
	pr3 := mustPRState(t, "#3", "Feature C", "feat-b", "feat-c", true, 0, nil)

	reader := &mockPRReader{prs: []domain.PRState{pr1, pr2, pr3}}
	emitter := &mockEmitter{}
	store := &mockStateReader{}

	// when
	dmails, err := runPreMergePipeline(context.Background(), "main", reader, store, emitter, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(dmails) != 1 {
		t.Fatalf("expected 1 dmail, got: %d", len(dmails))
	}
	if dmails[0].Kind != domain.KindImplFeedback {
		t.Errorf("expected kind %q, got %q", domain.KindImplFeedback, dmails[0].Kind)
	}
	if len(emitter.dmailsGenerated) != 1 {
		t.Errorf("expected 1 emitted dmail, got: %d", len(emitter.dmailsGenerated))
	}
	if emitter.prConvergenceCheckedData == nil {
		t.Fatal("expected pr_convergence.checked event")
	}
	if emitter.prConvergenceCheckedData.TotalOpenPRs != 3 {
		t.Errorf("expected TotalOpenPRs=3, got %d", emitter.prConvergenceCheckedData.TotalOpenPRs)
	}
}

func TestRunPreMergePipeline_withConflict(t *testing.T) {
	// given
	pr1 := mustPRState(t, "#10", "Conflict PR", "main", "feat-x", false, 3, []string{"file.go"})

	reader := &mockPRReader{prs: []domain.PRState{pr1}}
	emitter := &mockEmitter{}
	store := &mockStateReader{}

	// when
	dmails, err := runPreMergePipeline(context.Background(), "main", reader, store, emitter, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(dmails) != 1 {
		t.Fatalf("expected 1 dmail, got: %d", len(dmails))
	}
	if dmails[0].Severity != domain.SeverityHigh {
		t.Errorf("expected severity %q, got %q", domain.SeverityHigh, dmails[0].Severity)
	}
	if emitter.prConvergenceCheckedData == nil {
		t.Fatal("expected pr_convergence.checked event")
	}
	if emitter.prConvergenceCheckedData.ConflictPRs != 1 {
		t.Errorf("expected ConflictPRs=1, got %d", emitter.prConvergenceCheckedData.ConflictPRs)
	}
}

func TestRunPreMergePipeline_manyPRs(t *testing.T) {
	// given: 50 PRs — 3 chains + orphans
	var prs []domain.PRState
	prs = append(prs, mustPRState(t, "#1", "Feature A", "main", "feat-a", true, 1, nil))
	prs = append(prs, mustPRState(t, "#2", "Feature B", "feat-a", "feat-b", true, 0, nil))
	prs = append(prs, mustPRState(t, "#3", "Feature X", "main", "feat-x", false, 5, []string{"api.go"}))
	prs = append(prs, mustPRState(t, "#4", "Feature Y", "main", "feat-y", true, 0, nil))
	prs = append(prs, mustPRState(t, "#5", "Feature Z", "feat-y", "feat-z", true, 0, nil))
	prs = append(prs, mustPRState(t, "#6", "Feature W", "feat-z", "feat-w", true, 0, nil))
	for i := 7; i <= 50; i++ {
		prs = append(prs, mustPRState(t, fmt.Sprintf("#%d", i),
			fmt.Sprintf("Orphan %d", i), "develop",
			fmt.Sprintf("orphan-%d", i), true, 0, nil))
	}

	reader := &mockPRReader{prs: prs}
	emitter := &mockEmitter{}
	store := &mockStateReader{}

	// when
	dmails, err := runPreMergePipeline(context.Background(), "main", reader, store, emitter, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(dmails) != 3 {
		t.Errorf("expected 3 dmails, got %d", len(dmails))
	}
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
}

func TestRunPreMergePipeline_singleMergeablePR(t *testing.T) {
	// given: 1 PR, mergeable, not behind
	pr1 := mustPRState(t, "#5", "Clean PR", "main", "feat-clean", true, 0, nil)

	reader := &mockPRReader{prs: []domain.PRState{pr1}}
	emitter := &mockEmitter{}
	store := &mockStateReader{}

	// when
	dmails, err := runPreMergePipeline(context.Background(), "main", reader, store, emitter, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(dmails) != 0 {
		t.Fatalf("expected 0 dmails, got: %d", len(dmails))
	}
	if emitter.prConvergenceCheckedData == nil {
		t.Fatal("expected pr_convergence.checked event")
	}
	if emitter.prConvergenceCheckedData.DMails != 0 {
		t.Errorf("expected DMails=0, got %d", emitter.prConvergenceCheckedData.DMails)
	}
}
