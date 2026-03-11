package session_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// fakeRunner is a test double for ClaudeRunner.
type fakeRunner struct {
	calls    int
	failN    int
	output   string
	lastOpts port.RunConfig
}

func (f *fakeRunner) Run(_ context.Context, _ string, _ io.Writer, opts ...port.RunOption) (string, error) {
	f.calls++
	f.lastOpts = port.ApplyOptions(opts...)
	if f.calls <= f.failN {
		return "", errors.New("claude exit: non-zero")
	}
	return f.output, nil
}

func TestRetryRunner_SucceedsFirstAttempt(t *testing.T) {
	// given
	inner := &fakeRunner{output: "ok"}
	runner := &session.RetryRunner{
		Inner:       inner,
		MaxAttempts: 3,
		BaseDelay:   0,
		Logger:      &domain.NopLogger{},
	}

	// when
	result, err := runner.Run(context.Background(), "test", io.Discard)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
	if inner.calls != 1 {
		t.Errorf("expected 1 call, got %d", inner.calls)
	}
}

func TestRetryRunner_RetriesAndSucceeds(t *testing.T) {
	// given: fails 2 times, succeeds on 3rd
	inner := &fakeRunner{failN: 2, output: "success"}
	runner := &session.RetryRunner{
		Inner:       inner,
		MaxAttempts: 3,
		BaseDelay:   0,
		Logger:      &domain.NopLogger{},
	}

	// when
	result, err := runner.Run(context.Background(), "test", io.Discard)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "success" {
		t.Errorf("expected 'success', got %q", result)
	}
	if inner.calls != 3 {
		t.Errorf("expected 3 calls, got %d", inner.calls)
	}
}

func TestRetryRunner_ExhaustsRetries(t *testing.T) {
	// given: always fails
	inner := &fakeRunner{failN: 100, output: "never"}
	runner := &session.RetryRunner{
		Inner:       inner,
		MaxAttempts: 2,
		BaseDelay:   0,
		Logger:      &domain.NopLogger{},
	}

	// when
	_, err := runner.Run(context.Background(), "test", io.Discard)

	// then
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if inner.calls != 2 {
		t.Errorf("expected 2 calls, got %d", inner.calls)
	}
}

func TestRetryRunner_NoRetryOnCancel(t *testing.T) {
	// given: cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	inner := &fakeRunner{failN: 100}
	runner := &session.RetryRunner{
		Inner:       inner,
		MaxAttempts: 3,
		BaseDelay:   0,
		Logger:      &domain.NopLogger{},
	}

	// when
	_, err := runner.Run(ctx, "test", io.Discard)

	// then
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if inner.calls > 1 {
		t.Errorf("expected at most 1 call on cancel, got %d", inner.calls)
	}
}

func TestRetryRunner_ForwardsOptions(t *testing.T) {
	// given
	inner := &fakeRunner{output: "ok"}
	runner := &session.RetryRunner{
		Inner:       inner,
		MaxAttempts: 1,
		BaseDelay:   0,
		Logger:      &domain.NopLogger{},
	}

	// when
	_, err := runner.Run(context.Background(), "test", io.Discard,
		port.WithAllowedTools("Read"))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inner.lastOpts.AllowedTools) != 1 || inner.lastOpts.AllowedTools[0] != "Read" {
		t.Errorf("expected forwarded allowed tools, got %v", inner.lastOpts.AllowedTools)
	}
}

func TestRetryRunner_MaxAttemptsLessThanOne_DefaultsToOne(t *testing.T) {
	// given
	inner := &fakeRunner{failN: 100}
	runner := &session.RetryRunner{
		Inner:       inner,
		MaxAttempts: 0,
		BaseDelay:   0,
		Logger:      &domain.NopLogger{},
	}

	// when
	_, err := runner.Run(context.Background(), "test", io.Discard)

	// then
	if err == nil {
		t.Fatal("expected error")
	}
	if inner.calls != 1 {
		t.Errorf("expected 1 call (defaulted from 0), got %d", inner.calls)
	}
}

// Verify RetryRunner satisfies the ClaudeRunner interface at compile time.
var _ port.ClaudeRunner = (*session.RetryRunner)(nil)

// Ensure unused imports are used.
var _ = time.Second
