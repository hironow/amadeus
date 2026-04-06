package domain_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestRetryBudgetTracker_NewWithInitial(t *testing.T) {
	// given
	tracker := domain.NewRetryBudgetTracker(3)

	// then
	if tracker.Remaining() != 3 {
		t.Errorf("expected remaining=3, got %d", tracker.Remaining())
	}
	if tracker.Exhausted() {
		t.Error("expected not exhausted")
	}
}

func TestRetryBudgetTracker_ConsumeDecrementsRemaining(t *testing.T) {
	// given
	tracker := domain.NewRetryBudgetTracker(2)

	// when
	ok := tracker.Consume()

	// then
	if !ok {
		t.Error("expected Consume to return true when budget > 0")
	}
	if tracker.Remaining() != 1 {
		t.Errorf("expected remaining=1, got %d", tracker.Remaining())
	}
}

func TestRetryBudgetTracker_ConsumeReturnsFalseWhenExhausted(t *testing.T) {
	// given
	tracker := domain.NewRetryBudgetTracker(1)
	tracker.Consume() // consume the only one

	// when
	ok := tracker.Consume()

	// then
	if ok {
		t.Error("expected Consume to return false when exhausted")
	}
	if !tracker.Exhausted() {
		t.Error("expected exhausted after consuming all budget")
	}
	if tracker.Remaining() != 0 {
		t.Errorf("expected remaining=0, got %d", tracker.Remaining())
	}
}

func TestRetryBudgetTracker_ResetRestoresBudget(t *testing.T) {
	// given
	tracker := domain.NewRetryBudgetTracker(2)
	tracker.Consume()
	tracker.Consume()

	// when
	tracker.Reset(5)

	// then
	if tracker.Remaining() != 5 {
		t.Errorf("expected remaining=5 after reset, got %d", tracker.Remaining())
	}
	if tracker.Exhausted() {
		t.Error("expected not exhausted after reset")
	}
}

func TestRetryBudgetTracker_SnapshotReturnsRemaining(t *testing.T) {
	// given
	tracker := domain.NewRetryBudgetTracker(3)
	tracker.Consume()

	// when
	snapshot := tracker.Snapshot()

	// then
	if snapshot != 2 {
		t.Errorf("expected snapshot=2, got %d", snapshot)
	}
}

func TestRetryBudgetTracker_ZeroInitialIsImmediatelyExhausted(t *testing.T) {
	// given
	tracker := domain.NewRetryBudgetTracker(0)

	// then
	if !tracker.Exhausted() {
		t.Error("expected exhausted with initial=0")
	}
	if tracker.Remaining() != 0 {
		t.Errorf("expected remaining=0, got %d", tracker.Remaining())
	}
}
