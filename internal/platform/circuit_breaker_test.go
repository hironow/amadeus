package platform_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
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
	info := domain.ClassifyProviderError(domain.ProviderClaudeCode, "You've hit your limit")
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
			info := domain.ClassifyProviderError(domain.ProviderClaudeCode, tc.stderr)
			cb.RecordProviderError(info)

			if !cb.IsOpen() {
				t.Fatalf("expected open for %q", tc.name)
			}
		})
	}
}

func TestCircuitBreaker_DoesNotTripOnNormalError(t *testing.T) {
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	info := domain.ClassifyProviderError(domain.ProviderClaudeCode, "some normal error")
	cb.RecordProviderError(info)

	if err := cb.Allow(context.Background()); err != nil {
		t.Fatalf("expected nil (closed), got %v", err)
	}
}

func TestCircuitBreaker_ResetsOnSuccess(t *testing.T) {
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	info := domain.ClassifyProviderError(domain.ProviderClaudeCode, "You've hit your limit")
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
	info := domain.ClassifyProviderError(domain.ProviderClaudeCode,
		"You've hit your limit · resets Apr 3 at 1pm (Asia/Tokyo)")
	cb.RecordProviderError(info)

	resetAt := cb.ResetAt()
	if resetAt.IsZero() {
		t.Fatal("expected non-zero ResetAt")
	}
}

func TestCircuitBreaker_FallbackBackoffWhenNoResetTime(t *testing.T) {
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	info := domain.ClassifyProviderError(domain.ProviderClaudeCode, "You've hit your limit")
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

	info := domain.ClassifyProviderError(domain.ProviderClaudeCode, "overloaded")
	cb.RecordProviderError(info)

	if !cb.IsOpen() {
		t.Fatal("expected open after trip")
	}
}

func TestCircuitBreaker_CodexRateLimit(t *testing.T) {
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	info := domain.ClassifyProviderError(domain.ProviderCodex, "rate_limit_exceeded: too many requests")
	cb.RecordProviderError(info)

	if !cb.IsOpen() {
		t.Fatal("expected open for Codex rate limit")
	}
}
