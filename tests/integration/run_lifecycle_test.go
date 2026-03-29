package integration_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// --- Test doubles for lifecycle integration tests ---

// lifecycleGit implements port.Git with static responses.
type lifecycleGit struct {
	branch string
	commit string
}

func (g *lifecycleGit) CurrentBranch() (string, error)                     { return g.branch, nil }
func (g *lifecycleGit) CurrentCommit() (string, error)                     { return g.commit, nil }
func (g *lifecycleGit) MergedPRsSince(_ string) ([]domain.MergedPR, error) { return nil, nil }
func (g *lifecycleGit) DiffSince(_ string) (string, error)                 { return "", nil }

var _ port.Git = (*lifecycleGit)(nil)

// lifecyclePRReader implements port.GitHubPRReader with configurable PRs.
type lifecyclePRReader struct {
	prs []domain.PRState
}

func (r *lifecyclePRReader) ListOpenPRs(_ context.Context, _ string) ([]domain.PRState, error) {
	return r.prs, nil
}

func (r *lifecyclePRReader) GetPRDiff(_ context.Context, _ string) (string, error) {
	return "", nil
}

var _ port.GitHubPRReader = (*lifecyclePRReader)(nil)

// --- Helpers ---

// setupGateDir initializes a .gate directory structure with real stores.
func setupGateDir(t *testing.T) (tmpDir, gateDir string, store *session.ProjectionStore, eventStore port.EventStore, outbox *session.SQLiteOutboxStore) {
	t.Helper()
	tmpDir = t.TempDir()
	gateDir = filepath.Join(tmpDir, ".gate")
	if err := session.InitGateDir(gateDir, &domain.NopLogger{}); err != nil {
		t.Fatalf("InitGateDir: %v", err)
	}

	store = session.NewProjectionStore(gateDir)
	eventStore = session.NewEventStore(gateDir, &domain.NopLogger{})

	var err error
	outbox, err = session.NewOutboxStoreForDir(gateDir)
	if err != nil {
		t.Fatalf("NewOutboxStoreForDir: %v", err)
	}
	t.Cleanup(func() { outbox.Close() })

	return tmpDir, gateDir, store, eventStore, outbox
}

// buildAmadeus creates a session.Amadeus with real stores and the given test doubles.
func buildAmadeus(
	tmpDir, gateDir string,
	store *session.ProjectionStore,
	eventStore port.EventStore,
	outbox *session.SQLiteOutboxStore,
	git port.Git,
	prReader port.GitHubPRReader,
) *session.Amadeus {
	projector := &session.Projector{Store: store, OutboxStore: outbox}
	return &session.Amadeus{
		Config:    domain.DefaultConfig(),
		Store:     store,
		Events:    eventStore,
		Projector: projector,
		Git:       git,
		RepoDir:   tmpDir,
		Logger:    &domain.NopLogger{},
		DataOut:   io.Discard,
		Approver:  &port.AutoApprover{},
		Notifier:  &port.NopNotifier{},
		Metrics:   &port.NopPolicyMetrics{},
		PRReader:  prReader,
	}
}

// findEventByType returns true if at least one event with the given type exists.
func findEventByType(events []domain.Event, eventType domain.EventType) bool {
	for _, ev := range events {
		if ev.Type == eventType {
			return true
		}
	}
	return false
}

// countEventsByType returns the number of events matching the given type.
func countEventsByType(events []domain.Event, eventType domain.EventType) int {
	count := 0
	for _, ev := range events {
		if ev.Type == eventType {
			count++
		}
	}
	return count
}

// writeReportDMailToInbox creates a valid report D-Mail file in the inbox directory.
func writeReportDMailToInbox(t *testing.T, gateDir string) {
	t.Helper()
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "report-001",
		Kind:          domain.KindReport,
		Description:   "Test expedition report from paintress",
		Severity:      domain.SeverityMedium,
		Action:        domain.ActionResolve,
		Priority:      3,
		Targets:       []string{"amadeus"},
		Body:          "## Expedition Report\n\nCompleted implementation of feature X.\n",
	}
	data, err := domain.MarshalDMail(dmail)
	if err != nil {
		t.Fatalf("MarshalDMail: %v", err)
	}
	inboxPath := filepath.Join(gateDir, "inbox", "report-001.md")
	if err := os.WriteFile(inboxPath, data, 0o644); err != nil {
		t.Fatalf("write inbox dmail: %v", err)
	}
}

// --- Tests ---

func TestRunLifecycle_StartAndStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// given: temp dir with real .gate structure, real event store, real projector
	tmpDir, gateDir, store, eventStore, outbox := setupGateDir(t)
	git := &lifecycleGit{branch: "main", commit: "abc1234"}
	a := buildAmadeus(tmpDir, gateDir, store, eventStore, outbox, git, nil)

	// Context with timeout to let the loop start and then cancel
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	opts := domain.RunOptions{
		CheckOptions: domain.CheckOptions{Quiet: true},
	}

	rp, err := domain.NewRepoPath(tmpDir)
	if err != nil {
		t.Fatalf("NewRepoPath: %v", err)
	}
	cmd := domain.NewExecuteRunCommand(rp, "")

	// when: run the daemon loop (exits when context is cancelled)
	err = usecase.Run(ctx, cmd, opts, a, domain.DefaultConfig(),
		&domain.NopLogger{}, &port.NopNotifier{}, &port.NopPolicyMetrics{}, nil, nil)

	// then: no error
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// then: verify run.started and run.stopped events in event store
	events, _, loadErr := eventStore.LoadAll()
	if loadErr != nil {
		t.Fatalf("LoadAll: %v", loadErr)
	}

	if !findEventByType(events, domain.EventRunStarted) {
		t.Error("missing run.started event in event store")
	}
	if !findEventByType(events, domain.EventRunStopped) {
		t.Error("missing run.stopped event in event store")
	}

	// Verify ordering: run.started must appear before run.stopped
	startedIdx := -1
	stoppedIdx := -1
	for i, ev := range events {
		if ev.Type == domain.EventRunStarted && startedIdx == -1 {
			startedIdx = i
		}
		if ev.Type == domain.EventRunStopped && stoppedIdx == -1 {
			stoppedIdx = i
		}
	}
	if startedIdx >= 0 && stoppedIdx >= 0 && startedIdx >= stoppedIdx {
		t.Errorf("run.started (idx=%d) should appear before run.stopped (idx=%d)", startedIdx, stoppedIdx)
	}
}

func TestRunLifecycle_InboxTrigger(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// given: temp dir with real .gate structure, report D-Mail in inbox
	tmpDir, gateDir, store, eventStore, outbox := setupGateDir(t)
	writeReportDMailToInbox(t, gateDir)

	git := &lifecycleGit{branch: "develop", commit: "def5678"}

	// PRReader with 3 PRs forming a chain (triggers pre-merge pipeline)
	pr1, err := domain.NewPRState("#1", "Feature A", "develop", "feat-a", true, 2, nil, nil, "")
	if err != nil {
		t.Fatalf("NewPRState: %v", err)
	}
	pr2, err := domain.NewPRState("#2", "Feature B", "feat-a", "feat-b", true, 0, nil, nil, "")
	if err != nil {
		t.Fatalf("NewPRState: %v", err)
	}
	pr3, err := domain.NewPRState("#3", "Feature C", "feat-b", "feat-c", true, 0, nil, nil, "")
	if err != nil {
		t.Fatalf("NewPRState: %v", err)
	}
	prReader := &lifecyclePRReader{prs: []domain.PRState{pr1, pr2, pr3}}

	a := buildAmadeus(tmpDir, gateDir, store, eventStore, outbox, git, prReader)

	// Short context timeout: enough for one scan + processing
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	opts := domain.RunOptions{
		CheckOptions: domain.CheckOptions{Quiet: true},
	}

	rp, err := domain.NewRepoPath(tmpDir)
	if err != nil {
		t.Fatalf("NewRepoPath: %v", err)
	}
	cmd := domain.NewExecuteRunCommand(rp, "")

	// when: run the daemon loop
	err = usecase.Run(ctx, cmd, opts, a, domain.DefaultConfig(),
		&domain.NopLogger{}, &port.NopNotifier{}, &port.NopPolicyMetrics{}, prReader, store)

	// then: no error
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// then: verify events in the real event store
	events, _, loadErr := eventStore.LoadAll()
	if loadErr != nil {
		t.Fatalf("LoadAll: %v", loadErr)
	}

	// Verify run lifecycle events
	if !findEventByType(events, domain.EventRunStarted) {
		t.Error("missing run.started event")
	}
	if !findEventByType(events, domain.EventRunStopped) {
		t.Error("missing run.stopped event")
	}

	// Verify inbox was consumed
	inboxConsumedCount := countEventsByType(events, domain.EventInboxConsumed)
	if inboxConsumedCount < 1 {
		t.Errorf("expected at least 1 inbox.consumed event, got %d", inboxConsumedCount)
	}

	// Verify PR convergence was checked (pre-merge pipeline ran)
	prConvergenceCount := countEventsByType(events, domain.EventPRConvergenceChecked)
	if prConvergenceCount < 1 {
		t.Errorf("expected at least 1 pr_convergence.checked event, got %d", prConvergenceCount)
	}

	// Verify inbox directory is empty (consumed)
	inboxEntries, err := os.ReadDir(filepath.Join(gateDir, "inbox"))
	if err != nil {
		t.Fatalf("ReadDir inbox: %v", err)
	}
	if len(inboxEntries) != 0 {
		t.Errorf("expected empty inbox after consumption, got %d file(s)", len(inboxEntries))
	}

	// Verify D-Mail was archived
	archiveEntries, err := os.ReadDir(filepath.Join(gateDir, "archive"))
	if err != nil {
		t.Fatalf("ReadDir archive: %v", err)
	}
	hasArchivedReport := false
	for _, e := range archiveEntries {
		if e.Name() == "report-001.md" {
			hasArchivedReport = true
			break
		}
	}
	if !hasArchivedReport {
		t.Error("expected report-001.md to be archived after inbox consumption")
	}
}
