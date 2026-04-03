package platform_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness/verifier"
	"github.com/hironow/amadeus/internal/platform"
)

func TestCircuitBreaker_AllowWhenClosed(t *testing.T) {
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	if err := cb.Allow(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCircuitBreaker_TripsOnRateLimit_BlocksUntilCancelled(t *testing.T) {
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	info := verifier.ClassifyProviderError(domain.ProviderClaudeCode, "You've hit your limit")
	cb.RecordProviderError(info)

	// Allow blocks when OPEN; use short deadline to verify blocking behavior
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := cb.Allow(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded (blocking), got %v", err)
	}
	if !cb.IsOpen() {
		t.Fatal("expected still open after cancelled wait")
	}
}

func TestCircuitBreaker_TripsOnServerError(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
	}{
		{"overloaded", "Anthropic API is overloaded"},
		{"529", "Error: 529 overloaded"},
		{"500", "Error: 500 Internal Server Error"},
		{"502", "Error: 502 Bad Gateway"},
		{"503", "Error: 503 Service Unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cb := platform.NewCircuitBreaker(&domain.NopLogger{})
			info := verifier.ClassifyProviderError(domain.ProviderClaudeCode, tc.stderr)
			cb.RecordProviderError(info)

			if !cb.IsOpen() {
				t.Fatalf("expected open for %q", tc.name)
			}
		})
	}
}

func TestCircuitBreaker_DoesNotTripOnNormalError(t *testing.T) {
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	info := verifier.ClassifyProviderError(domain.ProviderClaudeCode, "some normal error")
	cb.RecordProviderError(info)

	if err := cb.Allow(context.Background()); err != nil {
		t.Fatalf("expected nil (closed), got %v", err)
	}
}

func TestCircuitBreaker_ResetsOnSuccess(t *testing.T) {
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	info := verifier.ClassifyProviderError(domain.ProviderClaudeCode, "You've hit your limit")
	cb.RecordProviderError(info)

	cb.RecordSuccess()

	if err := cb.Allow(context.Background()); err != nil {
		t.Fatalf("expected nil after reset, got %v", err)
	}
}

func TestCircuitBreaker_RespectsContextCancellation(t *testing.T) {
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	info := domain.ProviderErrorInfo{Kind: domain.ProviderErrorRateLimit}
	cb.RecordProviderError(info)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := cb.Allow(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestCircuitBreaker_BlocksAndResumesAfterBackoff(t *testing.T) {
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	// Trip with server error (no reset time → uses backoff)
	info := domain.ProviderErrorInfo{Kind: domain.ProviderErrorServer}
	cb.RecordProviderError(info)

	// Allow should block then transition to HALF_OPEN when backoff elapses.
	// We can't wait 30s in a test, so reset externally.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cb.RecordSuccess()
	}()

	err := cb.Allow(context.Background())
	if err != nil {
		t.Fatalf("expected nil after recovery, got %v", err)
	}
}

func TestCircuitBreaker_ParsesResetTimeViaClassifier(t *testing.T) {
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	info := verifier.ClassifyProviderError(domain.ProviderClaudeCode,
		"You've hit your limit · resets Apr 3 at 1pm (Asia/Tokyo)")
	cb.RecordProviderError(info)

	resetAt := cb.ResetAt()
	if resetAt.IsZero() {
		t.Fatal("expected non-zero ResetAt")
	}
}

func TestCircuitBreaker_FallbackBackoffWhenNoResetTime(t *testing.T) {
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	info := verifier.ClassifyProviderError(domain.ProviderClaudeCode, "You've hit your limit")
	cb.RecordProviderError(info)

	if !cb.ResetAt().IsZero() {
		t.Fatalf("expected zero ResetAt, got %v", cb.ResetAt())
	}
	if !cb.IsOpen() {
		t.Fatal("expected open")
	}
}

func TestCircuitBreaker_IsOpen(t *testing.T) {
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	if cb.IsOpen() {
		t.Fatal("expected not open initially")
	}

	info := verifier.ClassifyProviderError(domain.ProviderClaudeCode, "overloaded")
	cb.RecordProviderError(info)

	if !cb.IsOpen() {
		t.Fatal("expected open after trip")
	}
}

func TestCircuitBreaker_CodexRateLimit(t *testing.T) {
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	info := verifier.ClassifyProviderError(domain.ProviderCodex, "rate_limit_exceeded: too many requests")
	cb.RecordProviderError(info)

	if !cb.IsOpen() {
		t.Fatal("expected open for Codex rate limit")
	}
}

func TestCircuitBreaker_ConcurrentAllow_AllBlockAndResume(t *testing.T) {
	// given — CB tripped
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	cb.RecordProviderError(domain.ProviderErrorInfo{Kind: domain.ProviderErrorServer})

	// when — 10 goroutines call Allow, then RecordSuccess unblocks them
	const n = 10
	errs := make(chan error, n)
	for range n {
		go func() {
			errs <- cb.Allow(context.Background())
		}()
	}

	// Unblock after a short delay
	time.Sleep(30 * time.Millisecond)
	cb.RecordSuccess()

	// then — all should return nil (no deadlock, no race)
	for range n {
		select {
		case err := <-errs:
			if err != nil {
				t.Errorf("expected nil, got %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout: goroutine did not unblock")
		}
	}
}

func TestCircuitBreaker_AllowBlocksThenRecordSuccessUnblocks(t *testing.T) {
	// given — CB tripped with long backoff (no resetAt)
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	cb.RecordProviderError(domain.ProviderErrorInfo{Kind: domain.ProviderErrorServer})

	done := make(chan error, 1)
	go func() {
		done <- cb.Allow(context.Background())
	}()

	// when — RecordSuccess from another goroutine
	time.Sleep(30 * time.Millisecond)
	cb.RecordSuccess()

	// then — Allow should unblock with nil
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("deadlock: Allow did not unblock after RecordSuccess")
	}
}

func TestCircuitBreaker_FailedProbeReTripsWithDoubledBackoff(t *testing.T) {
	// given — trip once
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	cb.RecordProviderError(domain.ProviderErrorInfo{Kind: domain.ProviderErrorServer})

	// when — reset, then trip again
	cb.RecordSuccess()
	cb.RecordProviderError(domain.ProviderErrorInfo{Kind: domain.ProviderErrorServer})

	// then — should be open again
	if !cb.IsOpen() {
		t.Fatal("expected open after re-trip")
	}

	// Verify it's blocking (backoff should be doubled)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := cb.Allow(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected blocking (DeadlineExceeded), got %v", err)
	}
}

func TestCircuitBreaker_BackoffCapsAtMax(t *testing.T) {
	// given — trip many times to exceed max backoff
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	for range 20 {
		cb.RecordProviderError(domain.ProviderErrorInfo{Kind: domain.ProviderErrorServer})
	}

	// then — should still be open, not panic or overflow
	if !cb.IsOpen() {
		t.Fatal("expected open after many trips")
	}
}

func TestRecordCircuitBreaker_ClassifiesFromErrorMessage_WhenStderrEmpty(t *testing.T) {
	// This tests the recordCircuitBreaker function's fallback behavior:
	// when stderr is empty, it uses err.Error() for classification.
	// We test this via the verifier.ClassifyProviderError path directly.
	info := verifier.ClassifyProviderError(domain.ProviderClaudeCode, "claude exit: exit status 1\nYou've hit your limit")
	if !info.IsTrip() {
		t.Fatal("expected trip from error message containing rate limit text")
	}
	if info.Kind != domain.ProviderErrorRateLimit {
		t.Fatalf("expected RateLimit, got %v", info.Kind)
	}
}
