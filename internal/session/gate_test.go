package session_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	amadeus "github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/session"
)

// --- Fake implementations for gate tests ---

type fakeGit struct {
	commit string
	prs    []amadeus.MergedPR
	diff   string
}

func (g *fakeGit) CurrentCommit() (string, error)                        { return g.commit, nil }
func (g *fakeGit) MergedPRsSince(_ string) ([]amadeus.MergedPR, error)   { return g.prs, nil }
func (g *fakeGit) DiffSince(_ string) (string, error)                    { return g.diff, nil }

type fakeClaude struct {
	response string
}

func (c *fakeClaude) Run(_ context.Context, _ string) ([]byte, error) {
	return []byte(c.response), nil
}

type fakeStateReader struct {
	latest    amadeus.CheckResult
	dmailSeq  int
	allDMails []amadeus.DMail
}

func (s *fakeStateReader) LoadLatest() (amadeus.CheckResult, error) {
	return s.latest, nil
}
func (s *fakeStateReader) ScanInbox() ([]amadeus.DMail, error) {
	return nil, nil
}
func (s *fakeStateReader) NextDMailName(_ amadeus.DMailKind) (string, error) {
	s.dmailSeq++
	return fmt.Sprintf("test-dmail-%03d", s.dmailSeq), nil
}
func (s *fakeStateReader) LoadAllDMails() ([]amadeus.DMail, error) {
	return s.allDMails, nil
}
func (s *fakeStateReader) LoadConsumed() ([]amadeus.ConsumedRecord, error) {
	return nil, nil
}
func (s *fakeStateReader) LoadSyncState() (amadeus.SyncState, error) {
	return amadeus.SyncState{}, nil
}

type fakeEventStore struct {
	events []amadeus.Event
}

func (e *fakeEventStore) Append(events ...amadeus.Event) error {
	e.events = append(e.events, events...)
	return nil
}
func (e *fakeEventStore) LoadAll() ([]amadeus.Event, error) {
	return e.events, nil
}
func (e *fakeEventStore) LoadSince(_ time.Time) ([]amadeus.Event, error) {
	return e.events, nil
}

type fakeProjector struct {
	applied []amadeus.Event
}

func (p *fakeProjector) Apply(event amadeus.Event) error {
	p.applied = append(p.applied, event)
	return nil
}
func (p *fakeProjector) Rebuild(_ []amadeus.Event) error {
	return nil
}

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

func newGateTestAmadeus(t *testing.T, approver amadeus.Approver, notifier amadeus.Notifier) *session.Amadeus {
	t.Helper()
	// Create a minimal gate dir with required structure
	root := t.TempDir()
	gateDir := filepath.Join(root, ".gate")
	for _, sub := range []string{".run", "archive", "outbox", "inbox"} {
		os.MkdirAll(filepath.Join(gateDir, sub), 0o755)
	}

	events := &fakeEventStore{}
	projector := &fakeProjector{}

	return &session.Amadeus{
		Config: amadeus.Config{
			Lang: "en",
			FullCheck: amadeus.FullCheckConfig{
				Interval: 10,
			},
		},
		Store: &fakeStateReader{
			latest: amadeus.CheckResult{
				CheckedAt:  time.Now().Add(-1 * time.Hour),
				Commit:     "abc123",
				Type:       amadeus.CheckTypeDiff,
				Divergence: 0.05,
			},
		},
		Events:    events,
		Projector: projector,
		Git: &fakeGit{
			commit: "def456",
			prs:    []amadeus.MergedPR{{Number: "#1", Title: "test PR"}},
			diff:   "diff --git a/file.go b/file.go\n--- a/file.go\n+++ b/file.go\n@@ -1 +1 @@\n-old\n+new",
		},
		Claude:   &fakeClaude{response: claudeResponseWithDrift()},
		RepoDir:  root,
		Logger:   amadeus.NewLogger(io.Discard, false),
		Approver: approver,
		Notifier: notifier,
	}
}

// --- Gate Tests ---

func TestRunCheck_GateApproved_GeneratesDMails(t *testing.T) {
	// given: AutoApprover always approves
	notifier := &fakeNotifier{}
	a := newGateTestAmadeus(t, &amadeus.AutoApprover{}, notifier)

	// when
	err := a.RunCheck(context.Background(), session.CheckOptions{})

	// then: should return DriftError (dmails generated)
	var driftErr *amadeus.DriftError
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
	a := newGateTestAmadeus(t, &denyApprover{}, &amadeus.NopNotifier{})
	a.Events = events
	a.Projector = projector

	// when
	err := a.RunCheck(context.Background(), session.CheckOptions{})

	// then: should return nil (no DriftError — D-Mails skipped)
	if err != nil {
		t.Fatalf("expected nil error (gate denied), got: %v", err)
	}

	// check.completed event should still be emitted (ES invariant)
	found := false
	for _, ev := range events.events {
		if ev.Type == amadeus.EventCheckCompleted {
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
		if ev.Type == amadeus.EventCheckCompleted {
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
	a := newGateTestAmadeus(t, &errorApprover{err: gateErr}, &amadeus.NopNotifier{})

	// when
	err := a.RunCheck(context.Background(), session.CheckOptions{})

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
	a := newGateTestAmadeus(t, nil, &amadeus.NopNotifier{})

	// when
	err := a.RunCheck(context.Background(), session.CheckOptions{})

	// then: should return DriftError (dmails generated, gate skipped)
	var driftErr *amadeus.DriftError
	if !errors.As(err, &driftErr) {
		t.Fatalf("expected DriftError, got: %v", err)
	}
	if driftErr.DMails == 0 {
		t.Error("expected DMails > 0")
	}
}
