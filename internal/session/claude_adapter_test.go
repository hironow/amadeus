package session_test

import (
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
