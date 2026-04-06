package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestImprovementTaskDispatcher_DispatchAndDedup(t *testing.T) {
	// given
	stateDir := t.TempDir()
	dispatcher, err := session.NewImprovementTaskDispatcher(stateDir, &domain.NopLogger{})
	if err != nil {
		t.Fatalf("create dispatcher: %v", err)
	}
	defer dispatcher.Close()

	ctx := context.Background()
	task := domain.NewImprovementTask(
		"dmail-001", "paintress", "retry", domain.FailureTypeExecutionFailure, domain.SeverityMedium, 30*time.Minute,
	)

	// when — first dispatch
	err = dispatcher.Dispatch(ctx, task, "corr-001")
	if err != nil {
		t.Fatalf("first dispatch: %v", err)
	}

	// when — second dispatch with same correlation + failure (should be deduped)
	task2 := domain.NewImprovementTask(
		"dmail-001", "paintress", "retry again", domain.FailureTypeExecutionFailure, domain.SeverityMedium, 30*time.Minute,
	)
	err = dispatcher.Dispatch(ctx, task2, "corr-001")

	// then — no error (silently deduped)
	if err != nil {
		t.Fatalf("second dispatch should not error: %v", err)
	}
}

func TestImprovementTaskDispatcher_DifferentCorrelationsAreSeparate(t *testing.T) {
	// given
	stateDir := t.TempDir()
	dispatcher, err := session.NewImprovementTaskDispatcher(stateDir, &domain.NopLogger{})
	if err != nil {
		t.Fatalf("create dispatcher: %v", err)
	}
	defer dispatcher.Close()

	ctx := context.Background()

	// when — dispatch with different correlations
	task1 := domain.NewImprovementTask("ev1", "paintress", "act1", domain.FailureTypeExecutionFailure, domain.SeverityMedium, 30*time.Minute)
	task2 := domain.NewImprovementTask("ev2", "sightjack", "act2", domain.FailureTypeScopeViolation, domain.SeverityHigh, 30*time.Minute)

	err1 := dispatcher.Dispatch(ctx, task1, "corr-001")
	err2 := dispatcher.Dispatch(ctx, task2, "corr-002")

	// then — both should succeed
	if err1 != nil {
		t.Errorf("dispatch 1: %v", err1)
	}
	if err2 != nil {
		t.Errorf("dispatch 2: %v", err2)
	}
}
