package session

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	amadeus "github.com/hironow/amadeus"
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
func RunReviewGate(ctx context.Context, reviewCmd, claudeCmd, model, dir string, timeoutSec int, logger *amadeus.Logger, budget ...int) (bool, error) {
	if strings.TrimSpace(reviewCmd) == "" {
		return true, nil
	}

	if logger == nil {
		logger = amadeus.NewLogger(nil, false)
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

	for cycle := 1; cycle <= maxCycles; cycle++ {
		if ctx.Err() != nil {
			return false, fmt.Errorf("review gate canceled: %w", ctx.Err())
		}

		logger.Info("Review gate: cycle %d/%d", cycle, maxReviewGateCycles)

		reviewCtx, reviewCancel := context.WithTimeout(ctx, reviewTimeout)
		result, err := RunReview(reviewCtx, reviewCmd, dir)
		reviewCancel()
		if err != nil {
			return false, fmt.Errorf("review gate cycle %d: %w", cycle, err)
		}

		if result.Passed {
			logger.Info("Review gate: passed")
			return true, nil
		}

		logger.Warn("Review gate: comments found (cycle %d/%d)", cycle, maxCycles)

		if cycle == maxCycles {
			break
		}
	}

	logger.Warn("Review gate: exhausted %d cycles, review not resolved", maxCycles)
	return false, nil
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
