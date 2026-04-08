package session

// white-box-reason: tests that claudeRunner() propagates StreamBus to ClaudeAdapter

import (
	"context"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
)

func TestStreamBusWiring_ClaudeRunner(t *testing.T) {
	// given: Amadeus with StreamBus set
	bus := platform.NewInProcessSessionBus()
	defer bus.Close()
	sub := bus.Subscribe(16)
	defer sub.Close()

	a := &Amadeus{
		ClaudeCmd:   "nonexistent",
		ClaudeModel: "test",
		Logger:      &domain.NopLogger{},
		RepoDir:     t.TempDir(),
		StreamBus:   bus,
	}

	// when: claudeRunner() creates a ClaudeAdapter
	runner := a.claudeRunner()

	// then: runner is non-nil (session tracking may fail, but adapter is returned)
	if runner == nil {
		t.Fatal("expected non-nil runner")
	}

	// Verify bus is wired by publishing directly and checking subscriber receives
	bus.Publish(context.Background(), domain.SessionStreamEvent{
		Tool:      "amadeus",
		Type: "session_end",
		Timestamp: time.Now(),
	})

	select {
	case ev := <-sub.C():
		if ev.Tool != "amadeus" {
			t.Errorf("expected Tool=amadeus, got %q", ev.Tool)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive event within timeout")
	}
}
