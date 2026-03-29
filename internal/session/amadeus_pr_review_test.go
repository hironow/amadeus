package session

// white-box-reason: tests unexported evaluatePRDiffs with mock port implementations for PR reader/writer

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// --- PR review test doubles ---

type fakePRReaderForReview struct {
	prs   []domain.PRState
	diffs map[string]string // prNumber -> diff
}

func (f *fakePRReaderForReview) ListOpenPRs(_ context.Context, _ string) ([]domain.PRState, error) {
	return f.prs, nil
}

func (f *fakePRReaderForReview) GetPRDiff(_ context.Context, prNumber string) (string, error) {
	if d, ok := f.diffs[prNumber]; ok {
		return d, nil
	}
	return "", nil
}

type fakePRWriterForReview struct {
	appliedLabels map[string][]string // prNumber -> []label
}

func newFakePRWriter() *fakePRWriterForReview {
	return &fakePRWriterForReview{appliedLabels: make(map[string][]string)}
}

func (f *fakePRWriterForReview) ApplyLabel(_ context.Context, prNumber, label string) error {
	f.appliedLabels[prNumber] = append(f.appliedLabels[prNumber], label)
	return nil
}

type fakeClaudeRunnerForReview struct {
	response string
}

func (f *fakeClaudeRunnerForReview) Run(_ context.Context, _ string, _ io.Writer, _ ...port.RunOption) (string, error) {
	return f.response, nil
}

// --- Tests ---

func TestEvaluatePRDiffs_SkipsReviewedCommit(t *testing.T) {
	// given: PR already has the review label for its current head SHA
	pr, _ := domain.NewPRState("#1", "Feature", "main", "feat-a", true, 0, nil,
		[]string{"amadeus:reviewed-abc12345"}, "abc12345def")
	reader := &fakePRReaderForReview{
		prs:   []domain.PRState{pr},
		diffs: map[string]string{"#1": "diff content"},
	}
	writer := newFakePRWriter()

	a := &Amadeus{
		PRReader: reader,
		PRWriter: writer,
		Logger:   &domain.NopLogger{},
	}

	// when
	dmails, err := a.evaluatePRDiffs(context.Background(), "main")

	// then: skipped (already reviewed this commit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dmails) != 0 {
		t.Errorf("expected 0 D-Mails (skipped), got %d", len(dmails))
	}
	if len(writer.appliedLabels) != 0 {
		t.Errorf("expected no labels applied, got %v", writer.appliedLabels)
	}
}

func TestEvaluatePRDiffs_ReEvaluatesAfterPush(t *testing.T) {
	// given: PR has review label for OLD commit, but head SHA has changed
	pr, _ := domain.NewPRState("#1", "Feature", "main", "feat-a", true, 0, nil,
		[]string{"amadeus:reviewed-old12345"}, "newsha789abcdef")
	reader := &fakePRReaderForReview{
		prs:   []domain.PRState{pr},
		diffs: map[string]string{"#1": "diff --git a/main.go b/main.go\n+new code"},
	}
	writer := newFakePRWriter()

	a := &Amadeus{
		PRReader:    reader,
		PRWriter:    writer,
		Logger:      &testLogger{t: t},
		Claude:      &fakeClaudeRunnerForReview{response: `{"axes": {"structural": {"score": 10}}, "reasoning": "Minor issues", "dmails": []}`},
		ClaudeModel: "test-model",
		Config:      domain.Config{Lang: "en"},
		Emitter:     &nopReviewEmitter{},
	}

	// when
	dmails, err := a.evaluatePRDiffs(context.Background(), "main")

	// then: re-evaluated (new commit), label applied with new SHA
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	labels, ok := writer.appliedLabels["#1"]
	if !ok {
		t.Fatal("expected label applied to #1")
	}
	expectedLabel := "amadeus:reviewed-newsha78"
	found := false
	for _, l := range labels {
		if l == expectedLabel {
			found = true
		}
	}
	if !found {
		t.Errorf("expected label %q in %v", expectedLabel, labels)
	}
	_ = dmails
}

// testLogger logs to testing.T for debugging.
type testLogger struct {
	t *testing.T
}

func (l *testLogger) Debug(format string, args ...interface{}) { l.t.Logf("[DEBUG] "+format, args...) }
func (l *testLogger) Info(format string, args ...interface{})  { l.t.Logf("[INFO] "+format, args...) }
func (l *testLogger) Warn(format string, args ...interface{})  { l.t.Logf("[WARN] "+format, args...) }
func (l *testLogger) OK(format string, args ...interface{})    { l.t.Logf("[OK] "+format, args...) }
func (l *testLogger) Error(format string, args ...interface{}) { l.t.Logf("[ERROR] "+format, args...) }

func TestEvaluatePRDiffs_NoPRReader_ReturnsNil(t *testing.T) {
	// given: no PRReader
	a := &Amadeus{
		PRReader: nil,
		Logger:   &domain.NopLogger{},
	}

	// when
	dmails, err := a.evaluatePRDiffs(context.Background(), "main")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dmails != nil {
		t.Errorf("expected nil, got %v", dmails)
	}
}

func TestEvaluatePRDiffs_NoPRWriter_SkipsLabel(t *testing.T) {
	// given: PRReader set but no PRWriter (label writes disabled)
	pr, _ := domain.NewPRState("#1", "Feature", "main", "feat-a", true, 0, nil,
		nil, "abc12345def")
	reader := &fakePRReaderForReview{
		prs:   []domain.PRState{pr},
		diffs: map[string]string{"#1": "diff content"},
	}

	a := &Amadeus{
		PRReader:    reader,
		PRWriter:    nil, // no writer
		Logger:      &domain.NopLogger{},
		Claude:      &fakeClaudeRunnerForReview{response: `{"axes": {"structural": {"score": 0}}, "reasoning": "OK", "dmails": []}`},
		ClaudeModel: "test-model",
		Config:      domain.Config{Lang: "en"},
		Emitter:     &nopReviewEmitter{},
	}

	// when
	dmails, err := a.evaluatePRDiffs(context.Background(), "main")

	// then: no error, evaluation runs but no label applied
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = dmails
}

// nopReviewEmitter is a minimal CheckEventEmitter for PR review tests.
type nopReviewEmitter struct{}

func (n *nopReviewEmitter) EmitInboxConsumed(_ domain.InboxConsumedData, _ time.Time) error {
	return nil
}
func (n *nopReviewEmitter) EmitForceFullNextSet(_, _ float64, _ time.Time) error { return nil }
func (n *nopReviewEmitter) EmitDMailGenerated(_ domain.DMail, _ time.Time) error { return nil }
func (n *nopReviewEmitter) EmitConvergenceDetected(_ domain.ConvergenceAlert, _ time.Time) error {
	return nil
}
func (n *nopReviewEmitter) EmitDMailCommented(_, _ string, _ time.Time) error { return nil }
func (n *nopReviewEmitter) EmitCheck(_ domain.CheckResult, _ time.Time) error { return nil }
func (n *nopReviewEmitter) EmitRunStarted(_ domain.RunStartedData, _ time.Time) error {
	return nil
}
func (n *nopReviewEmitter) EmitRunStopped(_ domain.RunStoppedData, _ time.Time) error {
	return nil
}
func (n *nopReviewEmitter) EmitPRConvergenceChecked(_ domain.PRConvergenceCheckedData, _ time.Time) error {
	return nil
}
