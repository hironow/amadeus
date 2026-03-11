// white-box-reason: tests unexported WaitForDMail function
package session

import (
	"context"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

type nopLogger struct{}

func (nopLogger) Info(_ string, _ ...any)  {}
func (nopLogger) Warn(_ string, _ ...any)  {}
func (nopLogger) OK(_ string, _ ...any)    {}
func (nopLogger) Error(_ string, _ ...any) {}
func (nopLogger) Debug(_ string, _ ...any) {}

func TestWaitForDMail_ArrivalReturnsTrue(t *testing.T) {
	// given
	ch := make(chan domain.DMail, 1)
	ch <- domain.DMail{Name: "test-dmail"}

	// when
	arrived, err := WaitForDMail(context.Background(), ch, time.Minute, nopLogger{})

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
	arrived, err := WaitForDMail(context.Background(), ch, 10*time.Millisecond, nopLogger{})

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
	arrived, err := WaitForDMail(ctx, ch, time.Minute, nopLogger{})

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
	arrived, err := WaitForDMail(context.Background(), ch, time.Minute, nopLogger{})

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
	arrived, err := WaitForDMail(context.Background(), ch, 0, nopLogger{})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !arrived {
		t.Error("expected arrived=true")
	}
}
