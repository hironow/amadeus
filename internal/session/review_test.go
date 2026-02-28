package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// === RunReview ===

func TestRunReview_EmptyCommand(t *testing.T) {
	// given
	ctx := context.Background()

	// when
	result, err := RunReview(ctx, "", "/tmp")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("empty command should pass")
	}
}

func TestRunReview_PassingReview(t *testing.T) {
	// given
	ctx := context.Background()
	dir := t.TempDir()

	// when
	result, err := RunReview(ctx, "echo all good", dir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("exit 0 should mean passed")
	}
}

func TestRunReview_FailingReview(t *testing.T) {
	// given
	ctx := context.Background()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "review.sh")
	os.WriteFile(scriptPath, []byte("#!/bin/bash\necho '[P1] naming issue'\nexit 1\n"), 0755)

	// when
	result, err := RunReview(ctx, scriptPath, dir)

	// then
	if err != nil {
		t.Fatalf("non-zero exit should return ReviewResult, not error: %v", err)
	}
	if result.Passed {
		t.Error("non-zero exit code should mean review did not pass")
	}
	if !strings.Contains(result.Comments, "naming issue") {
		t.Errorf("comments should contain output, got: %s", result.Comments)
	}
}

func TestRunReview_ContextCanceled(t *testing.T) {
	// given
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// when
	_, err := RunReview(ctx, "sleep 10", t.TempDir())

	// then
	if err == nil {
		t.Error("expected error on canceled context")
	}
}

func TestRunReview_RateLimitDetected(t *testing.T) {
	// given
	ctx := context.Background()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "review.sh")
	os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'rate limit exceeded'\nexit 1\n"), 0755)

	// when
	_, err := RunReview(ctx, scriptPath, dir)

	// then
	if err == nil {
		t.Error("rate limited review should return error")
	}
	if !strings.Contains(err.Error(), "rate") {
		t.Errorf("error should mention rate limit, got: %v", err)
	}
}

// === RunReviewGate ===

func TestRunReviewGate_EmptyCmd_Passes(t *testing.T) {
	// given
	ctx := context.Background()

	// when
	passed, err := RunReviewGate(ctx, "", t.TempDir(), 300, nil)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !passed {
		t.Error("empty command should pass")
	}
}

func TestRunReviewGate_PassingReview(t *testing.T) {
	// given
	ctx := context.Background()

	// when
	passed, err := RunReviewGate(ctx, "echo ok", t.TempDir(), 300, nil)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !passed {
		t.Error("passing review should return true")
	}
}

func TestRunReviewGate_FailsAfterMaxCycles(t *testing.T) {
	// given
	ctx := context.Background()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "review.sh")
	os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'style error'\nexit 1\n"), 0755)

	// when — no Claude runner, so fix attempts will fail and exhaust cycles
	passed, err := RunReviewGate(ctx, scriptPath, dir, 300, nil)

	// then
	if err != nil {
		t.Fatalf("exhausted cycles should not be error: %v", err)
	}
	if passed {
		t.Error("should not pass after all cycles fail")
	}
}

func TestRunReviewGate_RespectsTimeout(t *testing.T) {
	// given
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// when
	_, err := RunReviewGate(ctx, "sleep 10", t.TempDir(), 300, nil)

	// then
	if err == nil {
		t.Error("expected error on timeout")
	}
}

func TestRunReviewGate_BudgetExceeded(t *testing.T) {
	// given — budget=1, review always fails
	ctx := context.Background()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "review.sh")
	os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'error'\nexit 1\n"), 0755)

	// when
	passed, err := RunReviewGate(ctx, scriptPath, dir, 300, nil, 1)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed {
		t.Error("expected passed=false with budget=1 and failing review")
	}
}

func TestRunReviewGate_BudgetZeroUsesDefault(t *testing.T) {
	// given — budget=0 means default
	ctx := context.Background()

	// when
	passed, err := RunReviewGate(ctx, "echo ok", t.TempDir(), 300, nil, 0)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !passed {
		t.Error("expected passed=true")
	}
}
