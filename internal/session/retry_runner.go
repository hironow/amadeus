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
	Inner          port.ClaudeRunner
	MaxAttempts    int
	BaseDelay      time.Duration
	Timeout        time.Duration
	Logger         domain.Logger
	CircuitBreaker *platform.CircuitBreaker // optional: skip retries when circuit is open
}

// logger returns Logger if non-nil, otherwise a NopLogger.
func (r *RetryRunner) logger() domain.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return &domain.NopLogger{}
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

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		// Circuit breaker: skip remaining retries when rate-limited
		if r.CircuitBreaker != nil {
			if cbErr := r.CircuitBreaker.Allow(ctx); cbErr != nil {
				if lastErr != nil {
					return "", lastErr
				}
				return "", cbErr
			}
		}
		if attempt > 1 {
			shift := attempt - 2
			if shift > 30 {
				shift = 30
			}
			delay := r.BaseDelay * time.Duration(1<<shift)
			r.logger().Info("Retrying (%d/%d) after %v...", attempt, maxAttempts, delay)
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

// RunDetailed executes the inner runner with retry, returning the last RunResult.
func (r *RetryRunner) RunDetailed(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (port.RunResult, error) {
	// If inner supports DetailedRunner, use it to capture session ID.
	detailed, ok := r.Inner.(port.DetailedRunner)
	if !ok {
		text, err := r.Run(ctx, prompt, w, opts...)
		return port.RunResult{Text: text}, err
	}

	maxAttempts := r.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	var lastResult port.RunResult
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return lastResult, ctx.Err()
		}
		// Circuit breaker: skip remaining retries when rate-limited
		if r.CircuitBreaker != nil {
			if cbErr := r.CircuitBreaker.Allow(ctx); cbErr != nil {
				if lastErr != nil {
					return lastResult, lastErr
				}
				return lastResult, cbErr
			}
		}
		if attempt > 1 {
			shift := attempt - 2
			if shift > 30 {
				shift = 30
			}
			delay := r.BaseDelay * time.Duration(1<<shift)
			r.logger().Info("Retrying (%d/%d) after %v...", attempt, maxAttempts, delay)
			select {
			case <-ctx.Done():
				return lastResult, ctx.Err()
			case <-time.After(delay):
			}
		}
		result, err := detailed.RunDetailed(ctx, prompt, w, opts...)
		lastResult = result
		if err == nil {
			return result, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return result, err
		}
		span := trace.SpanFromContext(ctx)
		span.AddEvent("claude.retry",
			trace.WithAttributes(
				attribute.Int("claude.attempt", attempt),
				attribute.String("claude.error", platform.SanitizeUTF8(err.Error())),
			),
		)
	}
	return lastResult, fmt.Errorf("claude failed after %d attempts: %w", maxAttempts, lastErr)
}
