package session

// white-box-reason: session internals: tests unexported RunReview execution flow

import (
	"context"
	"os"
	"os/exec"
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
	passed, err := RunReviewGate(ctx, "", "", "", t.TempDir(), 300, nil)

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
	passed, err := RunReviewGate(ctx, "echo ok", "", "", t.TempDir(), 300, nil)

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
	passed, err := RunReviewGate(ctx, scriptPath, "", "", dir, 300, nil)

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
	_, err := RunReviewGate(ctx, "sleep 10", "", "", t.TempDir(), 300, nil)

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
	passed, err := RunReviewGate(ctx, scriptPath, "", "", dir, 300, nil, 1)

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
	passed, err := RunReviewGate(ctx, "echo ok", "", "", t.TempDir(), 300, nil, 0)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !passed {
		t.Error("expected passed=true")
	}
}

// === RunReviewGate fix cycle ===

func TestRunReviewGate_FixCycleExecuted(t *testing.T) {
	// given — review fails once, then passes after fix
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	callCount := filepath.Join(dir, "call-count")
	os.WriteFile(callCount, []byte("0"), 0644)

	// Review script: fail first call, pass second
	reviewScript := filepath.Join(dir, "review.sh")
	os.WriteFile(reviewScript, []byte(`#!/bin/bash
COUNT=$(cat `+callCount+`)
COUNT=$((COUNT + 1))
echo $COUNT > `+callCount+`
if [ $COUNT -eq 1 ]; then
  echo "fix this naming issue"
  exit 1
fi
exit 0
`), 0755)

	// Fake claude: just succeed (noop fix)
	fakeClaudeScript := filepath.Join(dir, "fake-claude.sh")
	os.WriteFile(fakeClaudeScript, []byte("#!/bin/bash\nexit 0\n"), 0755)

	ctx := context.Background()

	// when — review fail → fix (fake claude) → review pass
	passed, err := RunReviewGate(ctx, reviewScript, fakeClaudeScript, "opus", dir, 300, nil, 3)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !passed {
		t.Error("expected passed=true after fix cycle resolves review")
	}
}

func TestRunReviewGate_FixCycleExhausted(t *testing.T) {
	// given — review always fails, fix never resolves it
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	reviewScript := filepath.Join(dir, "review.sh")
	os.WriteFile(reviewScript, []byte("#!/bin/bash\necho 'persistent issue'\nexit 1\n"), 0755)

	fakeClaudeScript := filepath.Join(dir, "fake-claude.sh")
	os.WriteFile(fakeClaudeScript, []byte("#!/bin/bash\nexit 0\n"), 0755)

	ctx := context.Background()

	// when — all cycles exhausted
	passed, err := RunReviewGate(ctx, reviewScript, fakeClaudeScript, "opus", dir, 300, nil, 2)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed {
		t.Error("expected passed=false after exhausting all fix cycles")
	}
}

func TestRunReviewGate_ReviewCommentsPropagatedToFix(t *testing.T) {
	// given — review outputs specific comments, verify they reach the fix prompt
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	promptCapture := filepath.Join(dir, "captured-prompt.txt")

	reviewScript := filepath.Join(dir, "review.sh")
	os.WriteFile(reviewScript, []byte("#!/bin/bash\necho 'UNIQUE-REVIEW-COMMENT-XYZ-789'\nexit 1\n"), 0755)

	// Fake claude captures -p argument to file
	fakeClaudeScript := filepath.Join(dir, "fake-claude.sh")
	os.WriteFile(fakeClaudeScript, []byte(`#!/bin/bash
while [ $# -gt 0 ]; do
  if [ "$1" = "-p" ]; then
    echo "$2" > `+promptCapture+`
    break
  fi
  shift
done
exit 0
`), 0755)

	ctx := context.Background()

	// when — review fails, fix is called with review comments in prompt
	RunReviewGate(ctx, reviewScript, fakeClaudeScript, "opus", dir, 300, nil, 2)

	// then — captured prompt should contain the review comments
	captured, err := os.ReadFile(promptCapture)
	if err != nil {
		t.Fatalf("fix was not called (no captured prompt): %v", err)
	}
	if !strings.Contains(string(captured), "UNIQUE-REVIEW-COMMENT-XYZ-789") {
		t.Errorf("review comments not propagated to fix prompt, got: %s", string(captured))
	}
}

func TestRunReviewGate_FixFailure_ReturnsFalse(t *testing.T) {
	// given — review fails, fix also fails (claude exits non-zero)
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	reviewScript := filepath.Join(dir, "review.sh")
	os.WriteFile(reviewScript, []byte("#!/bin/bash\necho 'issue'\nexit 1\n"), 0755)

	fakeClaudeScript := filepath.Join(dir, "fake-claude.sh")
	os.WriteFile(fakeClaudeScript, []byte("#!/bin/bash\nexit 1\n"), 0755)

	ctx := context.Background()

	// when — fix fails
	passed, err := RunReviewGate(ctx, reviewScript, fakeClaudeScript, "opus", dir, 300, nil, 2)

	// then — should return false (not error)
	if err != nil {
		t.Fatalf("fix failure should not be infrastructure error: %v", err)
	}
	if passed {
		t.Error("expected passed=false when fix fails")
	}
}

// initTestGitRepo creates a minimal git repo for review fix tests (needs currentBranch).
func initTestGitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// === BuildReviewFixPrompt ===

func TestBuildReviewFixPrompt(t *testing.T) {
	// given
	branch := "feature/foo"
	comments := "fix the bug"

	// when
	prompt := BuildReviewFixPrompt(branch, comments)

	// then
	if !strings.Contains(prompt, "feature/foo") {
		t.Error("prompt should contain branch name")
	}
	if !strings.Contains(prompt, "fix the bug") {
		t.Error("prompt should contain comments")
	}
}

// === summarizeReview ===

func TestSummarizeReview_Short(t *testing.T) {
	// given
	input := "short comment"

	// when
	result := summarizeReview(input)

	// then
	if result != "short comment" {
		t.Errorf("expected 'short comment', got %q", result)
	}
}

func TestSummarizeReview_Truncates(t *testing.T) {
	// given
	long := strings.Repeat("x", 600)

	// when
	result := summarizeReview(long)

	// then
	if len([]rune(result)) > 520 {
		t.Error("should truncate long output")
	}
	if !strings.HasSuffix(result, "...(truncated)") {
		t.Error("should end with truncation marker")
	}
}
