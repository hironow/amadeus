package session

// white-box-reason: tests unexported run-loop internals (goroutine lifecycle, channel coordination, fsnotify integration)

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// --- PR test doubles (shared with pr_convergence tests) ---

type mockPRReader struct {
	prs []domain.PRState
	err error
}

func (m *mockPRReader) ListOpenPRs(_ context.Context, _ string) ([]domain.PRState, error) {
	return m.prs, m.err
}

func mustPRState(t *testing.T, number, title, base, head string, mergeable bool, behindBy int, conflicts []string) domain.PRState {
	t.Helper()
	pr, err := domain.NewPRState(number, title, base, head, mergeable, behindBy, conflicts)
	if err != nil {
		t.Fatalf("NewPRState: %v", err)
	}
	return pr
}

// testPRPipeline wraps a mockPRReader+emitter+store into a port.PRPipelineRunner for Run tests.
type testPRPipeline struct {
	reader  port.GitHubPRReader
	store   port.StateReader
	emitter port.CheckEventEmitter
	logger  domain.Logger
}

func (p *testPRPipeline) RunPreMergePipeline(ctx context.Context, integrationBranch string) ([]domain.DMail, error) {
	if p.reader == nil {
		return nil, nil
	}
	// Inline simplified pipeline for session-level tests.
	// Full business logic is tested in usecase/pr_convergence_test.go.
	prs, err := p.reader.ListOpenPRs(ctx, integrationBranch)
	if err != nil {
		return nil, err
	}
	if len(prs) == 0 {
		return nil, nil
	}
	report := domain.BuildPRConvergenceReport(integrationBranch, prs)
	var dmails []domain.DMail
	var conflictCount int
	now := time.Now().UTC()
	for _, chain := range report.Chains {
		needsAction := len(chain.PRs) > 1 || chain.HasConflict
		if !needsAction && len(chain.PRs) == 1 && chain.PRs[0].BehindBy() > 0 {
			needsAction = true
		}
		if !needsAction {
			continue
		}
		if chain.HasConflict {
			conflictCount++
		}
		name, nameErr := p.store.NextDMailName(domain.KindImplFeedback)
		if nameErr != nil {
			return dmails, nameErr
		}
		singleReport := domain.PRConvergenceReport{
			IntegrationBranch: integrationBranch,
			Chains:            []domain.PRChain{chain},
			TotalOpenPRs:      report.TotalOpenPRs,
		}
		dmail := domain.BuildConvergenceDMail(name, singleReport)
		if errs := domain.ValidateDMail(dmail); len(errs) > 0 {
			continue
		}
		_ = p.emitter.EmitDMailGenerated(dmail, now)
		dmails = append(dmails, dmail)
	}
	_ = p.emitter.EmitPRConvergenceChecked(domain.PRConvergenceCheckedData{
		IntegrationBranch: integrationBranch,
		TotalOpenPRs:      report.TotalOpenPRs,
		Chains:            len(report.Chains),
		ConflictPRs:       conflictCount,
		DMails:            len(dmails),
	}, now)
	return dmails, nil
}

// --- Test doubles for Run tests ---

// runEmitter records Run lifecycle events for assertion.
type runEmitter struct {
	mu                 sync.Mutex
	runStartedCalled   bool
	runStartedData     *domain.RunStartedData
	runStoppedCalled   bool
	runStoppedData     *domain.RunStoppedData
	inboxConsumed      []domain.InboxConsumedData
	dmailsGenerated    []domain.DMail
	checksEmitted      []domain.CheckResult
	prConvergenceCalls int
}

func (e *runEmitter) EmitRunStarted(data domain.RunStartedData, _ time.Time) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.runStartedCalled = true
	e.runStartedData = &data
	return nil
}

func (e *runEmitter) EmitRunStopped(data domain.RunStoppedData, _ time.Time) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.runStoppedCalled = true
	e.runStoppedData = &data
	return nil
}

func (e *runEmitter) EmitInboxConsumed(data domain.InboxConsumedData, _ time.Time) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.inboxConsumed = append(e.inboxConsumed, data)
	return nil
}

func (e *runEmitter) EmitForceFullNextSet(_, _ float64, _ time.Time) error { return nil }
func (e *runEmitter) EmitDMailGenerated(dmail domain.DMail, _ time.Time) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dmailsGenerated = append(e.dmailsGenerated, dmail)
	return nil
}
func (e *runEmitter) EmitConvergenceDetected(_ domain.ConvergenceAlert, _ time.Time) error {
	return nil
}
func (e *runEmitter) EmitDMailCommented(_, _ string, _ time.Time) error { return nil }
func (e *runEmitter) EmitCheck(result domain.CheckResult, _ time.Time) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.checksEmitted = append(e.checksEmitted, result)
	return nil
}
func (e *runEmitter) EmitPRConvergenceChecked(_ domain.PRConvergenceCheckedData, _ time.Time) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.prConvergenceCalls++
	return nil
}

// runGit implements port.Git for Run tests.
type runGit struct {
	branch string
	commit string
	err    error
}

func (g *runGit) CurrentBranch() (string, error) {
	if g.err != nil {
		return "", g.err
	}
	return g.branch, nil
}

func (g *runGit) CurrentCommit() (string, error) {
	return g.commit, nil
}

func (g *runGit) MergedPRsSince(_ string) ([]domain.MergedPR, error) {
	return nil, nil
}

func (g *runGit) DiffSince(_ string) (string, error) {
	return "", nil
}

// Compile-time check
var _ port.Git = (*runGit)(nil)

// runStore is a mock StateReader that returns D-Mails on a controlled schedule.
type runStore struct {
	mu       sync.Mutex
	scanSeq  int
	scanPlan [][]domain.DMail // each entry is the result for successive ScanInbox calls
}

func (s *runStore) ScanInbox(_ context.Context) ([]domain.DMail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scanSeq >= len(s.scanPlan) {
		return nil, nil
	}
	result := s.scanPlan[s.scanSeq]
	s.scanSeq++
	return result, nil
}

func (s *runStore) LoadLatest() (domain.CheckResult, error) {
	return domain.CheckResult{}, nil
}

func (s *runStore) NextDMailName(_ domain.DMailKind) (string, error) {
	return "test-dmail-001", nil
}

func (s *runStore) LoadAllDMails() ([]domain.DMail, error) {
	return nil, nil
}

func (s *runStore) LoadConsumed() ([]domain.ConsumedRecord, error) {
	return nil, nil
}

func (s *runStore) LoadSyncState() (domain.SyncState, error) {
	return domain.SyncState{}, nil
}

// runState implements port.CheckStateProvider as a no-op for Run tests.
type runState struct{}

func (s *runState) ShouldFullCheck(_ bool) bool           { return false }
func (s *runState) ForceFullNext() bool                   { return false }
func (s *runState) SetForceFullNext(_ bool)               {}
func (s *runState) ShouldPromoteToFull(_, _ float64) bool { return false }
func (s *runState) AdvanceCheckCount(_, _ bool)            {}
func (s *runState) Restore(_ domain.CheckResult)          {}

// feedInbox creates a buffered channel pre-loaded with the given D-Mails.
func feedInbox(dmails ...domain.DMail) <-chan domain.DMail {
	ch := make(chan domain.DMail, len(dmails))
	for _, d := range dmails {
		ch <- d
	}
	return ch
}

// --- Tests ---

func TestRun_gracefulShutdown(t *testing.T) {
	// given: Amadeus with mock emitter, empty inbox channel, mock git
	emitter := &runEmitter{}
	git := &runGit{branch: "main", commit: "abc1234"}

	a := &Amadeus{
		Git:     git,
		Logger:  &domain.NopLogger{},
		InboxCh: make(chan domain.DMail), // unbuffered, blocks until ctx cancel
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a brief delay to let the loop start
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	opts := domain.RunOptions{}

	// when
	err := a.Run(ctx, opts, emitter, &runState{})

	// then: no error, run.started and run.stopped events emitted
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	emitter.mu.Lock()
	defer emitter.mu.Unlock()
	if !emitter.runStartedCalled {
		t.Error("expected run.started event to be emitted")
	}
	if emitter.runStartedData.IntegrationBranch != "main" {
		t.Errorf("expected integration branch %q, got %q", "main", emitter.runStartedData.IntegrationBranch)
	}
	if !emitter.runStoppedCalled {
		t.Error("expected run.stopped event to be emitted")
	}
	if emitter.runStoppedData.Reason != "signal" {
		t.Errorf("expected reason %q, got %q", "signal", emitter.runStoppedData.Reason)
	}
}

func TestRun_inboxTriggerPreMerge(t *testing.T) {
	// given: inbox channel delivers 1 report D-Mail
	reportDMail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "test-report-001",
		Kind:          domain.KindReport,
		Description:   "Test report",
	}

	emitter := &runEmitter{}
	git := &runGit{branch: "develop", commit: "def5678"}

	// PRReader with 3 PRs forming a chain
	pr1 := mustPRState(t, "#1", "Feature A", "develop", "feat-a", true, 2, nil)
	pr2 := mustPRState(t, "#2", "Feature B", "feat-a", "feat-b", true, 0, nil)
	pr3 := mustPRState(t, "#3", "Feature C", "feat-b", "feat-c", true, 0, nil)
	prReader := &mockPRReader{prs: []domain.PRState{pr1, pr2, pr3}}

	store := &runStore{}

	a := &Amadeus{
		Git:        git,
		Store:      store,
		PRReader:   prReader,
		PRPipeline: &testPRPipeline{reader: prReader, store: store, emitter: emitter, logger: &domain.NopLogger{}},
		Logger:     &domain.NopLogger{},
		InboxCh:    feedInbox(reportDMail),
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after enough time for processing
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	opts := domain.RunOptions{}

	// when
	err := a.Run(ctx, opts, emitter, &runState{})

	// then: no error
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	emitter.mu.Lock()
	defer emitter.mu.Unlock()

	// Verify inbox consumed event
	if len(emitter.inboxConsumed) != 1 {
		t.Fatalf("expected 1 inbox consumed event, got %d", len(emitter.inboxConsumed))
	}
	if emitter.inboxConsumed[0].Kind != domain.KindReport {
		t.Errorf("expected consumed kind %q, got %q", domain.KindReport, emitter.inboxConsumed[0].Kind)
	}

	// Verify PR convergence was checked: once on startup + once on D-Mail arrival
	if emitter.prConvergenceCalls != 2 {
		t.Errorf("expected 2 pr_convergence.checked events (initial + inbox), got %d", emitter.prConvergenceCalls)
	}

	// Verify at least 1 implementation-feedback D-Mail generated
	if len(emitter.dmailsGenerated) < 1 {
		t.Errorf("expected at least 1 dmail generated, got %d", len(emitter.dmailsGenerated))
	}
}

func TestRun_baseBranchOverridesCurrentBranch(t *testing.T) {
	// Regression: when opts.BaseBranch is set, integrationBranch must use it
	// instead of git.CurrentBranch(). Otherwise PR convergence finds 0 chains
	// because adjacency[currentBranch] is empty when PRs target BaseBranch.

	// given: BaseBranch="main", CurrentBranch="feat/something", PRs target "main"
	reportDMail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "test-report-base",
		Kind:          domain.KindReport,
		Description:   "Report triggering pre-merge",
	}

	emitter := &runEmitter{}
	git := &runGit{branch: "feat/something", commit: "aaa1111"}

	pr1 := mustPRState(t, "#10", "Feature X", "main", "feat-x", true, 1, nil)
	pr2 := mustPRState(t, "#11", "Feature Y", "feat-x", "feat-y", true, 0, nil)
	prReader := &mockPRReader{prs: []domain.PRState{pr1, pr2}}

	store := &runStore{}

	a := &Amadeus{
		Git:        git,
		Store:      store,
		PRReader:   prReader,
		PRPipeline: &testPRPipeline{reader: prReader, store: store, emitter: emitter, logger: &domain.NopLogger{}},
		Logger:     &domain.NopLogger{},
		InboxCh:    feedInbox(reportDMail),
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	opts := domain.RunOptions{
		BaseBranch: "main", // explicitly set, should override CurrentBranch
	}

	// when
	err := a.Run(ctx, opts, emitter, &runState{})

	// then
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	emitter.mu.Lock()
	defer emitter.mu.Unlock()

	// IntegrationBranch must be "main" (from BaseBranch), not "feat/something"
	if emitter.runStartedData == nil {
		t.Fatal("expected run.started event")
	}
	if emitter.runStartedData.IntegrationBranch != "main" {
		t.Errorf("expected integration branch %q, got %q", "main", emitter.runStartedData.IntegrationBranch)
	}

	// PR convergence must find chains (proves integrationBranch="main" was used)
	if emitter.prConvergenceCalls < 1 {
		t.Errorf("expected at least 1 pr_convergence.checked event, got %d", emitter.prConvergenceCalls)
	}

	// Implementation-feedback D-Mails must be generated from the chain
	if len(emitter.dmailsGenerated) < 1 {
		t.Errorf("expected at least 1 dmail generated, got %d", len(emitter.dmailsGenerated))
	}
}

func TestRun_noPRReaderSkipsPreMerge(t *testing.T) {
	// given: PRReader is nil, inbox channel delivers 1 report D-Mail
	reportDMail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "test-report-002",
		Kind:          domain.KindReport,
		Description:   "Another report",
	}

	emitter := &runEmitter{}
	git := &runGit{branch: "main", commit: "ghi9012"}

	store := &runStore{}

	a := &Amadeus{
		Git:      git,
		Store:    store,
		PRReader: nil, // no PR reader
		Logger:   &domain.NopLogger{},
		InboxCh:  feedInbox(reportDMail),
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	opts := domain.RunOptions{}

	// when
	err := a.Run(ctx, opts, emitter, &runState{})

	// then: no error
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	emitter.mu.Lock()
	defer emitter.mu.Unlock()

	// Inbox was consumed
	if len(emitter.inboxConsumed) != 1 {
		t.Fatalf("expected 1 inbox consumed event, got %d", len(emitter.inboxConsumed))
	}

	// But no PR convergence events (pre-merge skipped)
	if emitter.prConvergenceCalls != 0 {
		t.Errorf("expected 0 pr_convergence.checked events, got %d", emitter.prConvergenceCalls)
	}

	// And no D-Mails generated (no pre-merge, no post-merge)
	if len(emitter.dmailsGenerated) != 0 {
		t.Errorf("expected 0 dmails generated, got %d", len(emitter.dmailsGenerated))
	}
}

func TestRun_channelClosedEmitsRunStopped(t *testing.T) {
	// given: inbox channel that closes immediately
	emitter := &runEmitter{}
	git := &runGit{branch: "main", commit: "ccc3333"}
	ch := make(chan domain.DMail)
	close(ch)

	a := &Amadeus{
		Git:     git,
		Logger:  &domain.NopLogger{},
		InboxCh: ch,
	}

	opts := domain.RunOptions{}

	// when
	err := a.Run(context.Background(), opts, emitter, &runState{})

	// then
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	emitter.mu.Lock()
	defer emitter.mu.Unlock()

	if !emitter.runStoppedCalled {
		t.Fatal("expected run.stopped event")
	}
	if emitter.runStoppedData.Reason != "channel_closed" {
		t.Errorf("expected reason %q, got %q", "channel_closed", emitter.runStoppedData.Reason)
	}
}

func TestRun_monitorInboxError_emitsRunStopped(t *testing.T) {
	// given: InboxCh is nil, RepoDir is invalid → MonitorInbox should fail
	emitter := &runEmitter{}
	git := &runGit{branch: "main", commit: "eee5555"}

	a := &Amadeus{
		Git:     git,
		Logger:  &domain.NopLogger{},
		RepoDir: "/nonexistent/path/that/does/not/exist",
		// InboxCh is nil, so Run will call MonitorInbox which should fail
	}

	opts := domain.RunOptions{}

	// when
	err := a.Run(context.Background(), opts, emitter, &runState{})

	// then: should return error
	if err == nil {
		t.Fatal("expected error from MonitorInbox failure")
	}

	emitter.mu.Lock()
	defer emitter.mu.Unlock()

	// run.started should have been emitted before the error
	if !emitter.runStartedCalled {
		t.Error("expected run.started event before error")
	}

	// run.stopped should be emitted via defer on error path
	if !emitter.runStoppedCalled {
		t.Error("expected run.stopped event on error exit")
	}
	if emitter.runStoppedData != nil && emitter.runStoppedData.Reason != "error" {
		t.Errorf("expected reason %q, got %q", "error", emitter.runStoppedData.Reason)
	}
}

func TestRun_currentBranchErrorFallsBackToMain(t *testing.T) {
	// given: no BaseBranch set, CurrentBranch returns error → fallback to "main"
	emitter := &runEmitter{}
	git := &runGit{err: fmt.Errorf("not a git repo"), commit: "ddd4444"}

	a := &Amadeus{
		Git:     git,
		Logger:  &domain.NopLogger{},
		InboxCh: make(chan domain.DMail),
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	opts := domain.RunOptions{} // no BaseBranch

	// when
	err := a.Run(ctx, opts, emitter, &runState{})

	// then
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	emitter.mu.Lock()
	defer emitter.mu.Unlock()

	if emitter.runStartedData == nil {
		t.Fatal("expected run.started event")
	}
	if emitter.runStartedData.IntegrationBranch != "main" {
		t.Errorf("expected fallback integration branch %q, got %q", "main", emitter.runStartedData.IntegrationBranch)
	}
}
