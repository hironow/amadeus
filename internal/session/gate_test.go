package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// --- Fake implementations for gate tests ---

type fakeGit struct {
	commit string
	prs    []domain.MergedPR
	diff   string
}

func (g *fakeGit) CurrentCommit() (string, error)                     { return g.commit, nil }
func (g *fakeGit) CurrentBranch() (string, error)                     { return "main", nil }
func (g *fakeGit) MergedPRsSince(_ string) ([]domain.MergedPR, error) { return g.prs, nil }
func (g *fakeGit) DiffSince(_ string) (string, error)                 { return g.diff, nil }

type fakeClaude struct {
	response string
}

func (c *fakeClaude) Run(_ context.Context, _ string, _ io.Writer, _ ...port.RunOption) (string, error) {
	return c.response, nil
}

type fakeStateReader struct {
	latest    domain.CheckResult
	dmailSeq  int
	allDMails []domain.DMail
}

func (s *fakeStateReader) LoadLatest() (domain.CheckResult, error) {
	return s.latest, nil
}
func (s *fakeStateReader) ScanInbox(_ context.Context) ([]domain.DMail, error) {
	return nil, nil
}
func (s *fakeStateReader) NextDMailName(_ domain.DMailKind) (string, error) {
	s.dmailSeq++
	return fmt.Sprintf("test-dmail-%03d", s.dmailSeq), nil
}
func (s *fakeStateReader) LoadAllDMails() ([]domain.DMail, error) {
	return s.allDMails, nil
}
func (s *fakeStateReader) LoadConsumed() ([]domain.ConsumedRecord, error) {
	return nil, nil
}
func (s *fakeStateReader) LoadSyncState() (domain.SyncState, error) {
	return domain.SyncState{}, nil
}

type fakeEventStore struct {
	events []domain.Event
}

func (e *fakeEventStore) Append(events ...domain.Event) (domain.AppendResult, error) {
	e.events = append(e.events, events...)
	return domain.AppendResult{}, nil
}
func (e *fakeEventStore) LoadAll() ([]domain.Event, domain.LoadResult, error) {
	return e.events, domain.LoadResult{}, nil
}
func (e *fakeEventStore) LoadSince(_ time.Time) ([]domain.Event, domain.LoadResult, error) {
	return e.events, domain.LoadResult{}, nil
}

type fakeProjector struct {
	applied []domain.Event
}

func (p *fakeProjector) Apply(event domain.Event) error {
	p.applied = append(p.applied, event)
	return nil
}
func (p *fakeProjector) Rebuild(_ []domain.Event) error {
	return nil
}

// testCheckEventEmitter implements port.CheckEventEmitter for session tests.
// It wraps the aggregate + event store + projector without usecase import.
type testCheckEventEmitter struct {
	agg       *domain.CheckAggregate
	store     port.EventStore
	projector domain.EventApplier
}

func (e *testCheckEventEmitter) emit(events ...domain.Event) error {
	if e.store != nil {
		if _, err := e.store.Append(events...); err != nil {
			return err
		}
	}
	if e.projector != nil {
		for _, ev := range events {
			if err := e.projector.Apply(ev); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *testCheckEventEmitter) EmitInboxConsumed(data domain.InboxConsumedData, now time.Time) error {
	ev, err := e.agg.RecordInboxConsumed(data, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *testCheckEventEmitter) EmitForceFullNextSet(prevDiv, currDiv float64, now time.Time) error {
	ev, err := e.agg.RecordForceFullNextSet(prevDiv, currDiv, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *testCheckEventEmitter) EmitDMailGenerated(dmail domain.DMail, now time.Time) error {
	ev, err := e.agg.RecordDMailGenerated(dmail, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *testCheckEventEmitter) EmitConvergenceDetected(alert domain.ConvergenceAlert, now time.Time) error {
	ev, err := e.agg.RecordConvergenceDetected(alert, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *testCheckEventEmitter) EmitDMailCommented(dmailName, issueID string, now time.Time) error {
	ev, err := e.agg.RecordDMailCommented(dmailName, issueID, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *testCheckEventEmitter) EmitCheck(result domain.CheckResult, now time.Time) error {
	events, err := e.agg.RecordCheck(result, now)
	if err != nil {
		return err
	}
	return e.emit(events...)
}

func (e *testCheckEventEmitter) EmitRunStarted(data domain.RunStartedData, now time.Time) error {
	ev, err := e.agg.RecordRunStarted(data, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *testCheckEventEmitter) EmitRunStopped(data domain.RunStoppedData, now time.Time) error {
	ev, err := e.agg.RecordRunStopped(data, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *testCheckEventEmitter) EmitPRConvergenceChecked(data domain.PRConvergenceCheckedData, now time.Time) error {
	ev, err := e.agg.RecordPRConvergenceChecked(data, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

// testCheckStateProvider implements port.CheckStateManager for session tests.
type testCheckStateProvider struct {
	agg *domain.CheckAggregate
}

func (m *testCheckStateProvider) ShouldFullCheck(forceFlag bool) bool {
	return m.agg.ShouldFullCheck(forceFlag)
}
func (m *testCheckStateProvider) ForceFullNext() bool     { return m.agg.ForceFullNext() }
func (m *testCheckStateProvider) SetForceFullNext(v bool) { m.agg.SetForceFullNext(v) }
func (m *testCheckStateProvider) ShouldPromoteToFull(prev, curr float64) bool {
	return m.agg.ShouldPromoteToFull(prev, curr)
}
func (m *testCheckStateProvider) AdvanceCheckCount(fullCheck bool, wasForced bool) {
	m.agg.AdvanceCheckCount(fullCheck, wasForced)
}
func (m *testCheckStateProvider) Restore(result domain.CheckResult) { m.agg.Restore(result) }

type denyApprover struct{}

func (*denyApprover) RequestApproval(_ context.Context, _ string) (bool, error) {
	return false, nil
}

type errorApprover struct {
	err error
}

func (a *errorApprover) RequestApproval(_ context.Context, _ string) (bool, error) {
	return false, a.err
}

type fakeNotifier struct {
	called  bool
	title   string
	message string
}

func (n *fakeNotifier) Notify(_ context.Context, title, message string) error {
	n.called = true
	n.title = title
	n.message = message
	return nil
}

// --- Helpers ---

// claudeResponseWithDrift returns a canned Claude response with drift and one DMailCandidate.
func claudeResponseWithDrift() string {
	return `{
		"axes": {
			"adr_integrity": {"score": 30, "details": "drift detected"},
			"dod_fulfillment": {"score": 20, "details": "some issues"},
			"dependency_integrity": {"score": 10, "details": "ok"},
			"implicit_constraints": {"score": 15, "details": "mild drift"}
		},
		"dmails": [
			{
				"description": "ADR drift detected",
				"issues": ["TEST-1"],
				"detail": "Detailed feedback body",
				"targets": ["sightjack"]
			}
		],
		"reasoning": "test drift"
	}`
}

func newGateTestAmadeus(t *testing.T, approver port.Approver, notifier port.Notifier) *session.Amadeus {
	t.Helper()
	// Create a minimal gate dir with required structure
	root := t.TempDir()
	gateDir := filepath.Join(root, ".gate")
	for _, sub := range []string{".run", "archive", "outbox", "inbox"} {
		os.MkdirAll(filepath.Join(gateDir, sub), 0o755)
	}

	events := &fakeEventStore{}
	projector := &fakeProjector{}

	cfg := domain.Config{
		Lang: "en",
		FullCheck: domain.FullCheckConfig{
			Interval: 10,
		},
	}
	agg := domain.NewCheckAggregate(cfg)
	emitter := &testCheckEventEmitter{agg: agg, store: events, projector: projector}
	state := &testCheckStateProvider{agg: agg}
	a := &session.Amadeus{
		Config: cfg,
		Store: &fakeStateReader{
			latest: domain.CheckResult{
				CheckedAt:  time.Now().Add(-1 * time.Hour),
				Commit:     "abc123",
				Type:       domain.CheckTypeDiff,
				Divergence: 0.05,
			},
		},
		Events:    events,
		Projector: projector,
		Git: &fakeGit{
			commit: "def456",
			prs:    []domain.MergedPR{{Number: "#1", Title: "test PR"}},
			diff:   "diff --git a/file.go b/file.go\n--- a/file.go\n+++ b/file.go\n@@ -1 +1 @@\n-old\n+new",
		},
		Claude:   &fakeClaude{response: claudeResponseWithDrift()},
		RepoDir:  root,
		Logger:   platform.NewLogger(io.Discard, false),
		Approver: approver,
		Notifier: notifier,
		Emitter:  emitter,
		State:    state,
	}
	return a
}

// claudeResponseWithDriftAndAction returns a canned Claude response
// where the D-Mail candidate includes an explicit action field.
func claudeResponseWithDriftAndAction(action string) string {
	return fmt.Sprintf(`{
		"axes": {
			"adr_integrity": {"score": 30, "details": "drift detected"},
			"dod_fulfillment": {"score": 20, "details": "some issues"},
			"dependency_integrity": {"score": 10, "details": "ok"},
			"implicit_constraints": {"score": 15, "details": "mild drift"}
		},
		"dmails": [
			{
				"description": "ADR drift detected",
				"issues": ["TEST-1"],
				"detail": "Detailed feedback body",
				"targets": ["sightjack"],
				"action": %q
			}
		],
		"reasoning": "test drift"
	}`, action)
}

// extractDMailsFromEvents extracts DMails from dmail.generated events.
func extractDMailsFromEvents(t *testing.T, events []domain.Event) []domain.DMail {
	t.Helper()
	var dmails []domain.DMail
	for _, ev := range events {
		if ev.Type != domain.EventDMailGenerated {
			continue
		}
		var data domain.DMailGeneratedData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			t.Fatalf("unmarshal DMailGeneratedData: %v", err)
		}
		dmails = append(dmails, data.DMail)
	}
	return dmails
}

// --- Gate Tests ---

func TestRunCheck_GateApproved_GeneratesDMails(t *testing.T) {
	// given: AutoApprover always approves
	notifier := &fakeNotifier{}
	a := newGateTestAmadeus(t, &port.AutoApprover{}, notifier)

	// when
	err := a.RunCheck(context.Background(), domain.CheckOptions{}, nil, nil)

	// then: should return DriftError (dmails generated)
	var driftErr *domain.DriftError
	if !errors.As(err, &driftErr) {
		t.Fatalf("expected DriftError, got: %v", err)
	}
	if driftErr.DMails == 0 {
		t.Error("expected DMails > 0")
	}
	// Notifier should have been called
	if !notifier.called {
		t.Error("expected notifier to be called")
	}
}

func TestRunCheck_GateDenied_NoDMails(t *testing.T) {
	// given: denyApprover always denies
	events := &fakeEventStore{}
	projector := &fakeProjector{}
	a := newGateTestAmadeus(t, &denyApprover{}, &port.NopNotifier{})
	a.Events = events
	a.Projector = projector
	cfg := a.Config
	agg := domain.NewCheckAggregate(cfg)
	a.Emitter = &testCheckEventEmitter{agg: agg, store: events, projector: projector}
	a.State = &testCheckStateProvider{agg: agg}

	// when
	err := a.RunCheck(context.Background(), domain.CheckOptions{}, nil, nil)

	// then: should return nil (no DriftError — D-Mails skipped)
	if err != nil {
		t.Fatalf("expected nil error (gate denied), got: %v", err)
	}

	// check.completed event should still be emitted (ES invariant)
	found := false
	for _, ev := range events.events {
		if ev.Type == domain.EventCheckCompleted {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected check.completed event to be emitted even when gate denied")
	}

	// Projector should have applied the check.completed event
	projectorFound := false
	for _, ev := range projector.applied {
		if ev.Type == domain.EventCheckCompleted {
			projectorFound = true
			break
		}
	}
	if !projectorFound {
		t.Error("expected projector to have applied check.completed event")
	}
}

func TestRunCheck_GateError_FailsClosed(t *testing.T) {
	// given: errorApprover returns an error
	gateErr := errors.New("approval service unavailable")
	a := newGateTestAmadeus(t, &errorApprover{err: gateErr}, &port.NopNotifier{})

	// when
	err := a.RunCheck(context.Background(), domain.CheckOptions{}, nil, nil)

	// then: should fail closed (return the error)
	if err == nil {
		t.Fatal("expected error for gate failure")
	}
	if !errors.Is(err, gateErr) {
		// The error is wrapped, check for the message
		if got := err.Error(); got != "approval gate: approval service unavailable" {
			t.Errorf("expected wrapped gate error, got: %v", err)
		}
	}
}

func TestRunCheck_NilApprover_AutoApproves(t *testing.T) {
	// given: nil Approver should skip gate entirely (backward compatible)
	a := newGateTestAmadeus(t, nil, &port.NopNotifier{})

	// when
	err := a.RunCheck(context.Background(), domain.CheckOptions{}, nil, nil)

	// then: should return DriftError (dmails generated, gate skipped)
	var driftErr *domain.DriftError
	if !errors.As(err, &driftErr) {
		t.Fatalf("expected DriftError, got: %v", err)
	}
	if driftErr.DMails == 0 {
		t.Error("expected DMails > 0")
	}
}

func TestRunCheck_FeedbackDMail_DefaultAction_BasedOnSeverity(t *testing.T) {
	// given: Claude response without explicit action — severity-based default should apply.
	// With zero-value config thresholds, divergence 0.0 >= MediumMax 0.0 → SeverityHigh → ActionEscalate.
	events := &fakeEventStore{}
	projector := &fakeProjector{}
	a := newGateTestAmadeus(t, &port.AutoApprover{}, &port.NopNotifier{})
	a.Events = events
	a.Projector = projector
	cfg := a.Config
	agg := domain.NewCheckAggregate(cfg)
	a.Emitter = &testCheckEventEmitter{agg: agg, store: events, projector: projector}
	a.State = &testCheckStateProvider{agg: agg}

	// when
	err := a.RunCheck(context.Background(), domain.CheckOptions{}, nil, nil)

	// then: should return DriftError
	var driftErr *domain.DriftError
	if !errors.As(err, &driftErr) {
		t.Fatalf("expected DriftError, got: %v", err)
	}

	// Extract DMails from events and verify action
	dmails := extractDMailsFromEvents(t, events.events)
	if len(dmails) == 0 {
		t.Fatal("expected at least one feedback D-Mail in events")
	}
	for _, dmail := range dmails {
		if dmail.Action != domain.ActionEscalate {
			t.Errorf("expected default action %q for high severity, got %q", domain.ActionEscalate, dmail.Action)
		}
	}
}

func TestRunCheck_FeedbackDMail_ExplicitAction_FromCandidate(t *testing.T) {
	// given: Claude response with explicit action "retry" on the candidate
	events := &fakeEventStore{}
	projector := &fakeProjector{}
	a := newGateTestAmadeus(t, &port.AutoApprover{}, &port.NopNotifier{})
	a.Events = events
	a.Projector = projector
	cfg := a.Config
	agg := domain.NewCheckAggregate(cfg)
	a.Emitter = &testCheckEventEmitter{agg: agg, store: events, projector: projector}
	a.State = &testCheckStateProvider{agg: agg}
	a.Claude = &fakeClaude{response: claudeResponseWithDriftAndAction("retry")}

	// when
	err := a.RunCheck(context.Background(), domain.CheckOptions{}, nil, nil)

	// then: should return DriftError
	var driftErr *domain.DriftError
	if !errors.As(err, &driftErr) {
		t.Fatalf("expected DriftError, got: %v", err)
	}

	// Extract DMails from events and verify explicit action is preserved
	dmails := extractDMailsFromEvents(t, events.events)
	if len(dmails) == 0 {
		t.Fatal("expected at least one feedback D-Mail in events")
	}
	for _, dmail := range dmails {
		if dmail.Action != domain.ActionRetry {
			t.Errorf("expected explicit action %q from candidate, got %q", domain.ActionRetry, dmail.Action)
		}
	}
}
