package session_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase/port"
)

func TestClaudeAdapter_ImplementsProviderRunner(t *testing.T) {
	// given
	adapter := &session.ClaudeAdapter{
		ClaudeCmd:  "claude",
		Model:      "opus",
		TimeoutSec: 1980,
		Logger:     &domain.NopLogger{},
	}

	// then
	var _ port.ProviderRunner = adapter
	_ = adapter // use variable
}

// TestClaudeAdapter_RunDetailedReturnsErrMCPPivotDeprecated is the
// canonical post jun15 MCP pivot assertion (refs/issues/0027): the
// previous streambus / lifecycle tests exercised an exec path that
// has been removed. This single test pins the behavior callers can
// rely on — every invocation short-circuits with
// session.ErrMCPPivotDeprecated so operators are routed to the
// human-initiated claude code /review-gate skill instead.
func TestClaudeAdapter_RunDetailedReturnsErrMCPPivotDeprecated(t *testing.T) {
	// given
	adapter := &session.ClaudeAdapter{
		ClaudeCmd:  "claude",
		Model:      "opus",
		TimeoutSec: 10,
		Logger:     &domain.NopLogger{},
	}

	// when
	_, err := adapter.Run(context.Background(), "anything", io.Discard)

	// then
	if !errors.Is(err, session.ErrMCPPivotDeprecated) {
		t.Errorf("Run() error = %v, want ErrMCPPivotDeprecated", err)
	}
}
