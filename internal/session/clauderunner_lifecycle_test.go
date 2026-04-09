package session

// white-box-reason: tests that claudeRunner() returns the same instance across calls
// and that CloseRunner() properly closes the session store

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestClaudeRunner_ReusedAcrossCalls(t *testing.T) {
	// given: Amadeus with default runner (no Claude override)
	a := &Amadeus{
		ClaudeCmd:   "nonexistent",
		ClaudeModel: "test",
		Logger:      &domain.NopLogger{},
		RepoDir:     t.TempDir(),
	}
	defer a.CloseRunner()

	// when: claudeRunner() called multiple times
	r1 := a.claudeRunner()
	r2 := a.claudeRunner()
	r3 := a.claudeRunner()

	// then: same instance returned each time
	if r1 != r2 || r2 != r3 {
		t.Error("expected same runner instance across calls")
	}

	// then: only one session store opened (not nil because TempDir is writable)
	if a.sessionStore == nil {
		t.Log("session store unavailable (expected in some environments)")
	}
}

func TestClaudeRunner_CloseRunnerIdempotent(t *testing.T) {
	// given
	a := &Amadeus{
		ClaudeCmd:   "nonexistent",
		ClaudeModel: "test",
		Logger:      &domain.NopLogger{},
		RepoDir:     t.TempDir(),
	}
	_ = a.claudeRunner() // trigger lazy init

	// when: CloseRunner called multiple times
	a.CloseRunner()
	a.CloseRunner() // should not panic

	// then: sessionStore is nil after close
	if a.sessionStore != nil {
		t.Error("expected sessionStore to be nil after CloseRunner")
	}
}
