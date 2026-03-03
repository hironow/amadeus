package usecase

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	amadeus "github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestRunCheck_InvalidCommand(t *testing.T) {
	// given: empty RepoPath
	cmd := domain.ExecuteCheckCommand{RepoPath: ""}
	opts := amadeus.CheckOptions{}
	a := &session.Amadeus{
		Config: amadeus.DefaultConfig(),
		Logger: amadeus.NewLogger(nil, false),
	}

	// when
	err := RunCheck(context.Background(), cmd, opts, a)

	// then: command validation should fail
	if err == nil {
		t.Fatal("expected error for empty RepoPath")
	}
	if got := err.Error(); got != "command validation: RepoPath is required" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestRunCheck_AggregateAndDispatcherInjected(t *testing.T) {
	// given: valid command with minimal real deps (temp dir with .gate/)
	tmpDir := t.TempDir()
	gateDir := filepath.Join(tmpDir, ".gate")
	if err := os.MkdirAll(filepath.Join(gateDir, ".run"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := session.InitGateDir(gateDir); err != nil {
		t.Fatal(err)
	}

	store := session.NewProjectionStore(gateDir)
	eventStore := session.NewEventStore(gateDir)

	cmd := domain.ExecuteCheckCommand{RepoPath: tmpDir}
	opts := amadeus.CheckOptions{DryRun: true}
	a := &session.Amadeus{
		Config: amadeus.DefaultConfig(),
		Store:  store,
		Events: eventStore,
		Logger: amadeus.NewLogger(nil, false),
	}

	// pre-conditions
	if a.Aggregate != nil {
		t.Fatal("aggregate should be nil before RunCheck")
	}
	if a.Dispatcher != nil {
		t.Fatal("dispatcher should be nil before RunCheck")
	}

	// when: RunCheck will fail at Git operations (not configured), but wiring happens first
	_ = RunCheck(context.Background(), cmd, opts, a)

	// then: aggregate and dispatcher should have been injected
	if a.Aggregate == nil {
		t.Fatal("aggregate should be injected after RunCheck")
	}
	if a.Dispatcher == nil {
		t.Fatal("dispatcher should be injected after RunCheck")
	}
}
