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

// RetryRunner wraps a provider runner (ProviderRunner) with exponential backoff retry.
// Use the inner runner directly for non-idempotent operations.
// Timeout bounds the entire retry loop (not per-attempt).
type RetryRunner struct {
	Inner          port.ProviderRunner
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

// retryLoop executes fn with exponential backoff retry.
// The entire loop is bounded by Timeout. fn receives the current attempt number.
func (r *RetryRunner) retryLoop(ctx context.Context, fn func(ctx context.Context, attempt int) error) error {
	maxAttempts := r.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Circuit breaker: skip remaining retries when rate-limited
		if r.CircuitBreaker != nil {
			if cbErr := r.CircuitBreaker.Allow(ctx); cbErr != nil {
				attrs := []attribute.KeyValue{
					attribute.String("provider.error", platform.SanitizeUTF8(cbErr.Error())),
				}
				attrs = append(attrs, providerStateSpanAttrs(r.CircuitBreaker.Snapshot())...)
				trace.SpanFromContext(ctx).AddEvent("provider.blocked", trace.WithAttributes(attrs...))
				if lastErr != nil {
					return lastErr
				}
				return cbErr
			}
		}
		if attempt > 1 {
			shift := min(attempt-2, 30)
			delay := r.BaseDelay * time.Duration(1<<shift)
			r.logger().Info("Retrying (%d/%d) after %v...", attempt, maxAttempts, delay)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		err := fn(ctx, attempt)
		if err == nil {
			return nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return err
		}
		span := trace.SpanFromContext(ctx)
		attrs := []attribute.KeyValue{
			attribute.Int("provider.attempt", attempt),
			attribute.String("provider.error", platform.SanitizeUTF8(err.Error())),
		}
		if r.CircuitBreaker != nil {
			attrs = append(attrs, providerStateSpanAttrs(r.CircuitBreaker.Snapshot())...)
		}
		span.AddEvent("provider.retry", trace.WithAttributes(attrs...))
	}
	return fmt.Errorf("provider failed after %d attempts: %w", maxAttempts, lastErr)
}

// Run executes the inner runner with exponential backoff retry.
// The entire retry loop is bounded by Timeout.
func (r *RetryRunner) Run(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (string, error) {
	var output string
	err := r.retryLoop(ctx, func(ctx context.Context, _ int) error {
		var runErr error
		output, runErr = r.Inner.Run(ctx, prompt, w, opts...)
		return runErr
	})
	return output, err
}

// RunDetailed executes the inner runner with retry, returning the last RunResult.
func (r *RetryRunner) RunDetailed(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (port.RunResult, error) {
	// If inner supports DetailedRunner, use it to capture session ID.
	detailed, ok := r.Inner.(port.DetailedRunner)
	if !ok {
		text, err := r.Run(ctx, prompt, w, opts...)
		return port.RunResult{Text: text}, err
	}

	var lastResult port.RunResult
	err := r.retryLoop(ctx, func(ctx context.Context, _ int) error {
		result, runErr := detailed.RunDetailed(ctx, prompt, w, opts...)
		lastResult = result
		return runErr
	})
	return lastResult, err
}
