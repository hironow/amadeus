package usecase_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase"
	"github.com/hironow/amadeus/internal/usecase/port"
)

func TestBuildCheckEmitter_ReturnsEmitterAndState(t *testing.T) {
	// given: valid deps (temp dir with .gate/)
	tmpDir := t.TempDir()
	gateDir := filepath.Join(tmpDir, ".gate")
	if err := os.MkdirAll(filepath.Join(gateDir, ".run"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := session.InitGateDir(gateDir, &domain.NopLogger{}, ""); err != nil {
		t.Fatal(err)
	}

	store := session.NewProjectionStore(gateDir)
	eventStore := session.NewEventStore(gateDir, &domain.NopLogger{})

	cfg := domain.DefaultConfig()
	logger := platform.NewLogger(nil, false)
	a := &session.Amadeus{
		Config: cfg,
		Store:  store,
		Events: eventStore,
		Logger: logger,
	}

	// when
	emitter, state := usecase.BuildCheckEmitter(context.Background(), "check-", a, cfg, logger, &port.NopNotifier{}, &port.NopPolicyMetrics{}, &port.NopImprovementTaskDispatcher{})

	// then
	if emitter == nil {
		t.Fatal("emitter should not be nil")
	}
	if state == nil {
		t.Fatal("state should not be nil")
	}
}
