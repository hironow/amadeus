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

func (f *fakePRReaderForReview) GetPRMergeReadiness(_ context.Context, _ string) (*domain.PRMergeReadiness, error) {
	return nil, nil
}

type fakePRWriterForReview struct {
	appliedLabels map[string][]string // prNumber -> []label
	removedLabels map[string][]string // prNumber -> []label
	deletedLabels []string
}

func newFakePRWriter() *fakePRWriterForReview {
	return &fakePRWriterForReview{
		appliedLabels: make(map[string][]string),
		removedLabels: make(map[string][]string),
	}
}

func (f *fakePRWriterForReview) ApplyLabel(_ context.Context, prNumber, label string) error {
	f.appliedLabels[prNumber] = append(f.appliedLabels[prNumber], label)
	return nil
}

func (f *fakePRWriterForReview) RemoveLabel(_ context.Context, prNumber, label string) error {
	f.removedLabels[prNumber] = append(f.removedLabels[prNumber], label)
	return nil
}

func (f *fakePRWriterForReview) DeleteLabel(_ context.Context, label string) error {
	f.deletedLabels = append(f.deletedLabels, label)
	return nil
}

func (f *fakePRWriterForReview) MergePR(_ context.Context, _ string, _ domain.MergeMethod) error {
	return nil
}

func (f *fakePRWriterForReview) ClosePR(_ context.Context, _, _ string) error {
	return nil
}

type fakeProviderRunnerForReview struct {
	response string
}

func (f *fakeProviderRunnerForReview) Run(_ context.Context, _ string, _ io.Writer, _ ...port.RunOption) (string, error) {
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
		Claude:      &fakeProviderRunnerForReview{response: `{"axes": {"structural": {"score": 10}}, "reasoning": "Minor issues", "dmails": []}`},
		ClaudeModel: "test-model",
		Config:      domain.Config{Lang: "en"},
		Emitter:     &nopReviewEmitter{},
	}

	// when
	dmails, err := a.evaluatePRDiffs(context.Background(), "main")

	// then: re-evaluated (new commit), single label applied
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	labels, ok := writer.appliedLabels["#1"]
	if !ok {
		t.Fatal("expected label applied to #1")
	}
	found := false
	for _, l := range labels {
		if l == PRReviewLabel {
			found = true
		}
	}
	if !found {
		t.Errorf("expected label %q in %v", PRReviewLabel, labels)
	}

	// then: legacy label should have been removed from PR
	removed, ok := writer.removedLabels["#1"]
	if !ok || len(removed) == 0 {
		t.Fatal("expected legacy label removed from #1")
	}
	oldLabelRemoved := false
	for _, l := range removed {
		if l == "amadeus:reviewed-old12345" {
			oldLabelRemoved = true
		}
	}
	if !oldLabelRemoved {
		t.Errorf("expected legacy label 'amadeus:reviewed-old12345' in removed list %v", removed)
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
		Claude:      &fakeProviderRunnerForReview{response: `{"axes": {"structural": {"score": 0}}, "reasoning": "OK", "dmails": []}`},
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

func TestEvaluatePRDiffs_GoTaskboardScenario(t *testing.T) {
	// given: 7 open PRs targeting main, no review labels (first run)
	// This mirrors the real go-taskboard state as of 2026-03-29.
	prs := []domain.PRState{
		mustPRStateWithSHA(t, "#14", "feat(#5): add ErrInvalidStatus validation", "main", "feat/input-validation-cluster-w2-5", "a9c5e6e3"),
		mustPRStateWithSHA(t, "#15", "test: pagination boundary tests", "main", "feat/pagination-w1-s1-reproduction-test", "e6a3617a"),
		mustPRStateWithSHA(t, "#16", "fix: validate offset/limit query params", "main", "feat/cluster-w2-1-handler-validation", "411f4200"),
		mustPRStateWithSHA(t, "#17", "fix(#5): validate status string", "main", "feat/input-validation-cluster-w3-5", "0ba57d71"),
		mustPRStateWithSHA(t, "#18", "test: pagination bug cluster reproduction", "main", "feat/pagination-cluster-w1-1-investigation", "e6b39c3a"),
		mustPRStateWithSHA(t, "#19", "fix: add pagination input validation", "main", "feat/pagination-w2-1-input-validation", "b04b4e5c"),
		mustPRStateWithSHA(t, "#20", "feat(#12, #13): TaskRepository filter/aggregation", "main", "feat/task-api-cluster-w2-12-sqlite", "457e717f"),
	}

	diffs := make(map[string]string)
	for _, pr := range prs {
		diffs[pr.Number()] = "diff --git a/main.go b/main.go\n+// changes for " + pr.Title()
	}

	reader := &fakePRReaderForReview{prs: prs, diffs: diffs}
	writer := newFakePRWriter()

	claudeResponse := `{"axes": {"structural": {"score": 5}}, "reasoning": "Minor deviations", "dmails": []}`
	evalCount := 0
	countingClaude := &countingProviderRunner{
		response: claudeResponse,
		count:    &evalCount,
	}

	a := &Amadeus{
		PRReader:    reader,
		PRWriter:    writer,
		Logger:      &domain.NopLogger{},
		Claude:      countingClaude,
		ClaudeModel: "test-model",
		Config:      domain.Config{Lang: "en"},
		Emitter:     &nopReviewEmitter{},
	}

	// when: first run — all 7 PRs should be evaluated
	dmails, err := a.evaluatePRDiffs(context.Background(), "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// then
	if evalCount != 7 {
		t.Errorf("expected 7 Claude evaluations, got %d", evalCount)
	}
	if len(writer.appliedLabels) != 7 {
		t.Errorf("expected 7 PRs labeled, got %d", len(writer.appliedLabels))
	}
	// Each PR should have exactly one label: amadeus:reviewed
	for _, pr := range prs {
		labels, ok := writer.appliedLabels[pr.Number()]
		if !ok {
			t.Errorf("PR %s: expected label applied", pr.Number())
			continue
		}
		if len(labels) != 1 || labels[0] != PRReviewLabel {
			t.Errorf("PR %s: expected label %q, got %v", pr.Number(), PRReviewLabel, labels)
		}
	}

	// when: second run — all 7 should be SKIPPED (already labeled)
	// Simulate labeled state by recreating PRs with labels
	var labeledPRs []domain.PRState
	for _, pr := range prs {
		lp, _ := domain.NewPRState(pr.Number(), pr.Title(), pr.BaseBranch(), pr.HeadBranch(),
			pr.Mergeable(), pr.BehindBy(), pr.ConflictFiles(), []string{PRReviewLabel}, pr.HeadSHA())
		labeledPRs = append(labeledPRs, lp)
	}
	reader2 := &fakePRReaderForReview{prs: labeledPRs, diffs: diffs}
	evalCount2 := 0
	a2 := &Amadeus{
		PRReader:    reader2,
		PRWriter:    newFakePRWriter(),
		Logger:      &domain.NopLogger{},
		Claude:      &countingProviderRunner{response: claudeResponse, count: &evalCount2},
		ClaudeModel: "test-model",
		Config:      domain.Config{Lang: "en"},
		Emitter:     &nopReviewEmitter{},
	}
	dmails2, err2 := a2.evaluatePRDiffs(context.Background(), "main")
	if err2 != nil {
		t.Fatalf("second run error: %v", err2)
	}
	if evalCount2 != 0 {
		t.Errorf("second run: expected 0 evaluations (all skipped), got %d", evalCount2)
	}
	if len(dmails2) != 0 {
		t.Errorf("second run: expected 0 D-Mails, got %d", len(dmails2))
	}
	_ = dmails
}

func mustPRStateWithSHA(t *testing.T, number, title, base, head, sha string) domain.PRState {
	t.Helper()
	ps, err := domain.NewPRState(number, title, base, head, true, 0, nil, nil, sha)
	if err != nil {
		t.Fatalf("mustPRStateWithSHA: %v", err)
	}
	return ps
}

// countingProviderRunner counts invocations.
type countingProviderRunner struct {
	response string
	count    *int
}

func (c *countingProviderRunner) Run(_ context.Context, _ string, _ io.Writer, _ ...port.RunOption) (string, error) {
	*c.count++
	return c.response, nil
}

// nopReviewEmitter is a minimal CheckEventEmitter for PR review tests.
type nopReviewEmitter struct{}

func (n *nopReviewEmitter) EmitInboxConsumed(_ context.Context, _ domain.InboxConsumedData, _ time.Time) error {
	return nil
}
func (n *nopReviewEmitter) EmitForceFullNextSet(_ context.Context, _, _ float64, _ time.Time) error {
	return nil
}
func (n *nopReviewEmitter) EmitDMailGenerated(_ context.Context, _ domain.DMail, _ time.Time) error {
	return nil
}
func (n *nopReviewEmitter) EmitConvergenceDetected(_ context.Context, _ domain.ConvergenceAlert, _ time.Time) error {
	return nil
}
func (n *nopReviewEmitter) EmitDMailCommented(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}
func (n *nopReviewEmitter) EmitCheck(_ context.Context, _ domain.CheckResult, _ time.Time) error {
	return nil
}
func (n *nopReviewEmitter) EmitRunStarted(_ context.Context, _ domain.RunStartedData, _ time.Time) error {
	return nil
}
func (n *nopReviewEmitter) EmitRunStopped(_ context.Context, _ domain.RunStoppedData, _ time.Time) error {
	return nil
}
func (n *nopReviewEmitter) EmitPRConvergenceChecked(_ context.Context, _ domain.PRConvergenceCheckedData, _ time.Time) error {
	return nil
}
func (n *nopReviewEmitter) EmitPRMerged(_ context.Context, _ domain.PRMergedData, _ time.Time) error {
	return nil
}
func (n *nopReviewEmitter) EmitPRMergeSkipped(_ context.Context, _ domain.PRMergeSkippedData, _ time.Time) error {
	return nil
}
