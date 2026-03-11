package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/usecase/port"
)

const maxReviewGateCycles = 3

// ReviewResult holds the outcome of a code review execution.
type ReviewResult struct {
	Passed   bool   // true if no actionable comments were found
	Output   string // raw review output
	Comments string // extracted review comments (empty if passed)
}

// RunReview executes the review command and parses the output.
func RunReview(ctx context.Context, reviewCmd string, dir string) (*ReviewResult, error) {
	if strings.TrimSpace(reviewCmd) == "" {
		return &ReviewResult{Passed: true}, nil
	}

	cmd := exec.CommandContext(ctx, shellName(), shellFlag(), reviewCmd)
	cmd.Dir = dir
	cmd.WaitDelay = 1 * time.Second

	out, err := cmd.CombinedOutput()
	output := string(out)

	if ctx.Err() != nil {
		return nil, fmt.Errorf("review command canceled: %w", ctx.Err())
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if isRateLimited(output) {
				return nil, fmt.Errorf("review service rate/quota limited")
			}
			return &ReviewResult{
				Passed:   false,
				Output:   output,
				Comments: output,
			}, nil
		}
		return nil, fmt.Errorf("review command failed: %w\noutput: %s", err, summarizeReview(output))
	}

	return &ReviewResult{
		Passed: true,
		Output: output,
	}, nil
}

// RunReviewGate runs the review-fix cycle after check.
// budget <= 0 means use default (3).
// Returns (true, nil) if review passes or is skipped (empty reviewCmd).
// Returns (false, nil) if review fails after all cycles.
// Returns (false, err) on infrastructure errors.
func RunReviewGate(ctx context.Context, reviewCmd string, runner port.ClaudeRunner, dir string, timeoutSec int, logger domain.Logger, budget ...int) (bool, error) {
	ctx, span := platform.Tracer.Start(ctx, "amadeus.review")
	defer span.End()

	if strings.TrimSpace(reviewCmd) == "" {
		return true, nil
	}

	if logger == nil {
		logger = &domain.NopLogger{}
	}

	maxCycles := maxReviewGateCycles
	if len(budget) > 0 && budget[0] > 0 {
		maxCycles = budget[0]
	}

	if timeoutSec <= 0 {
		timeoutSec = 300
	}
	reviewTimeout := max(
		time.Duration(timeoutSec)*time.Second/time.Duration(maxCycles),
		30*time.Second,
	)

	var lastComments string
	for cycle := 1; cycle <= maxCycles; cycle++ {
		if ctx.Err() != nil {
			span.RecordError(ctx.Err())
			span.SetAttributes(attribute.String("error.stage", "amadeus.review"))
			return false, fmt.Errorf("review gate canceled: %w", ctx.Err())
		}

		span.SetAttributes(attribute.Int("review.cycle", cycle))
		logger.Info("Review gate: cycle %d/%d", cycle, maxCycles)

		reviewStart := time.Now()
		reviewCtx, reviewCancel := context.WithTimeout(ctx, reviewTimeout)
		result, err := RunReview(reviewCtx, reviewCmd, dir)
		reviewCancel()
		span.SetAttributes(attribute.Int64("review.exec_ms", time.Since(reviewStart).Milliseconds()))
		if err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.String("error.stage", "amadeus.review"))
			return false, fmt.Errorf("review gate cycle %d: %w", cycle, err)
		}

		if result.Passed {
			logger.Info("Review gate: passed")
			return true, nil
		}

		lastComments = result.Comments
		logger.Warn("Review gate: comments found (cycle %d/%d)", cycle, maxCycles)

		// Last cycle — no point running fix
		if cycle == maxCycles {
			break
		}

		// Run Claude --continue to fix review comments
		if err := runReviewFix(ctx, runner, dir, lastComments, timeoutSec, logger); err != nil {
			logger.Warn("Review fix failed: %v", err)
			return false, nil
		}
	}

	logger.Warn("Review gate: exhausted %d cycles, review not resolved", maxCycles)
	return false, nil
}

// runReviewFix runs Claude --continue to fix review comments.
func runReviewFix(ctx context.Context, runner port.ClaudeRunner, dir, comments string, timeoutSec int, logger domain.Logger) error {
	if runner == nil {
		return fmt.Errorf("no ClaudeRunner configured for review fix")
	}

	branch, err := currentBranch(ctx, dir)
	if err != nil {
		return fmt.Errorf("detect branch: %w", err)
	}

	prompt := BuildReviewFixPrompt(branch, comments)

	fixTimeout := time.Duration(timeoutSec) * time.Second
	if fixTimeout <= 0 {
		fixTimeout = 300 * time.Second
	}
	fixCtx, fixCancel := context.WithTimeout(ctx, fixTimeout)
	defer fixCancel()

	logger.Info("Review fix: running claude --continue in %s", dir)
	_, err = runner.Run(fixCtx, prompt, io.Discard, port.WithContinue(), port.WithWorkDir(dir))
	if err != nil {
		return fmt.Errorf("claude fix: %w", err)
	}
	return nil
}

// BuildReviewFixPrompt creates a focused prompt for fixing review comments.
func BuildReviewFixPrompt(branch string, comments string) string {
	return fmt.Sprintf(`You are on branch %s. A code review found the following issues:

%s

Fix all review comments above. Commit and push your changes.
Keep fixes focused — only address the review comments, do not refactor unrelated code.`, branch, comments)
}

// summarizeReview normalizes multi-line review output and truncates.
func summarizeReview(comments string) string {
	normalized := strings.Join(strings.Fields(comments), " ")
	const maxLen = 500
	runes := []rune(normalized)
	if len(runes) <= maxLen {
		return normalized
	}
	return string(runes[:maxLen]) + "...(truncated)"
}

// currentBranch returns the current git branch name.
func currentBranch(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func isRateLimited(output string) bool {
	lower := strings.ToLower(output)
	signals := []string{
		"rate limit",
		"rate_limit",
		"quota exceeded",
		"quota limit",
		"too many requests",
		"usage limit",
	}
	for _, s := range signals {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}
