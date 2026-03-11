package session

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/usecase/port"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RetryRunner wraps a ClaudeRunner with exponential backoff retry.
// Use the inner runner directly for non-idempotent operations.
// Timeout bounds the entire retry loop (not per-attempt).
type RetryRunner struct {
	Inner       port.ClaudeRunner
	MaxAttempts int
	BaseDelay   time.Duration
	Timeout     time.Duration
	Logger      domain.Logger
}

// Run executes the inner runner with exponential backoff retry.
// The entire retry loop is bounded by Timeout.
func (r *RetryRunner) Run(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (string, error) {
	maxAttempts := r.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	// Wrap the entire retry loop in a single timeout so total wall time
	// is bounded regardless of MaxAttempts.
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	logger := r.Logger
	if logger == nil {
		logger = &domain.NopLogger{}
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if attempt > 1 {
			shift := attempt - 2
			if shift > 30 {
				shift = 30
			}
			delay := r.BaseDelay * time.Duration(1<<shift)
			logger.Info("Retrying (%d/%d) after %v...", attempt, maxAttempts, delay)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}
		output, err := r.Inner.Run(ctx, prompt, w, opts...)
		if err == nil {
			return output, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return output, err
		}
		span := trace.SpanFromContext(ctx)
		span.AddEvent("claude.retry",
			trace.WithAttributes(
				attribute.Int("claude.attempt", attempt),
				attribute.String("claude.error", platform.SanitizeUTF8(err.Error())),
			),
		)
	}
	return "", fmt.Errorf("claude failed after %d attempts: %w", maxAttempts, lastErr)
}
