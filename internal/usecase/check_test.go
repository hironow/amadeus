package usecase

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase/port"
)

func TestRunCheck_InvalidCommand(t *testing.T) {
	// given: empty RepoPath
	cmd := domain.ExecuteCheckCommand{RepoPath: ""}
	opts := domain.CheckOptions{}
	cfg := domain.DefaultConfig()
	logger := platform.NewLogger(nil, false)
	a := &session.Amadeus{
		Config: cfg,
		Logger: logger,
	}

	// when
	err := RunCheck(context.Background(), cmd, opts, a, cfg, logger, &port.NopNotifier{}, &port.NopPolicyMetrics{})

	// then: command validation should fail
	if err == nil {
		t.Fatal("expected error for empty RepoPath")
	}
	if got := err.Error(); got != "command validation: RepoPath is required" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestRunCheck_EmitterAndStateInjected(t *testing.T) {
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
	eventStore := session.NewEventStore(gateDir, &domain.NopLogger{})

	cmd := domain.ExecuteCheckCommand{RepoPath: tmpDir}
	opts := domain.CheckOptions{DryRun: true}
	cfg := domain.DefaultConfig()
	logger := platform.NewLogger(nil, false)
	a := &session.Amadeus{
		Config: cfg,
		Store:  store,
		Events: eventStore,
		Logger: logger,
	}

	// pre-conditions
	if a.Emitter != nil {
		t.Fatal("emitter should be nil before RunCheck")
	}
	if a.State != nil {
		t.Fatal("state should be nil before RunCheck")
	}

	// when: RunCheck will fail at Git operations (not configured), but wiring happens first
	_ = RunCheck(context.Background(), cmd, opts, a, cfg, logger, &port.NopNotifier{}, &port.NopPolicyMetrics{})

	// then: emitter and state should have been injected
	if a.Emitter == nil {
		t.Fatal("emitter should be injected after RunCheck")
	}
	if a.State == nil {
		t.Fatal("state should be injected after RunCheck")
	}
}
