package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestWaitForDMail_ArrivalReturnsTrue(t *testing.T) {
	// given
	ch := make(chan domain.DMail, 1)
	ch <- domain.DMail{Name: "test-dmail"}

	// when
	arrived, err := session.WaitForDMail(context.Background(), ch, time.Minute, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !arrived {
		t.Error("expected arrived=true when D-Mail is on channel")
	}
}

func TestWaitForDMail_TimeoutReturnsFalse(t *testing.T) {
	// given
	ch := make(chan domain.DMail)

	// when
	arrived, err := session.WaitForDMail(context.Background(), ch, 10*time.Millisecond, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if arrived {
		t.Error("expected arrived=false on timeout")
	}
}

func TestWaitForDMail_CancelReturnsFalse(t *testing.T) {
	// given
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan domain.DMail)
	cancel() // cancel immediately

	// when
	arrived, err := session.WaitForDMail(ctx, ch, time.Minute, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if arrived {
		t.Error("expected arrived=false on context cancel")
	}
}

func TestWaitForDMail_ClosedChannelReturnsFalse(t *testing.T) {
	// given
	ch := make(chan domain.DMail)
	close(ch)

	// when
	arrived, err := session.WaitForDMail(context.Background(), ch, time.Minute, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if arrived {
		t.Error("expected arrived=false on closed channel")
	}
}

func TestWaitForDMail_ZeroTimeoutNoDeadline(t *testing.T) {
	// given: zero timeout = no timeout, but we send a D-Mail to unblock
	ch := make(chan domain.DMail, 1)
	ch <- domain.DMail{Name: "test"}

	// when
	arrived, err := session.WaitForDMail(context.Background(), ch, 0, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !arrived {
		t.Error("expected arrived=true")
	}
}

func TestWaitForDMail_ZeroTimeout_UsesMaxWaitDuration(t *testing.T) {
	// given — timeout=0 should use maxWaitDuration safety cap, not block forever
	cleanup := session.ExportSetMaxWaitDuration(20 * time.Millisecond)
	t.Cleanup(cleanup)
	ch := make(chan domain.DMail) // no D-Mail will arrive

	// when
	start := time.Now()
	arrived, err := session.WaitForDMail(context.Background(), ch, 0, &domain.NopLogger{})
	elapsed := time.Since(start)

	// then — should return via safety cap, not hang
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if arrived {
		t.Error("expected arrived=false on safety cap timeout")
	}
	if elapsed > 1*time.Second {
		t.Errorf("expected quick return via safety cap, took %s", elapsed)
	}
}
