package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// Compile-time check that GhPRWriter implements port.GitHubPRWriter.
var _ port.GitHubPRWriter = (*GhPRWriter)(nil)

// GhPRWriter implements GitHubPRWriter using the gh CLI.
type GhPRWriter struct {
	RepoDir string
}

// NewGhPRWriter creates a new GhPRWriter for the given repository directory.
func NewGhPRWriter(repoDir string) *GhPRWriter {
	return &GhPRWriter{RepoDir: repoDir}
}

// ApplyLabel adds a label to the given PR. Creates the label if it doesn't exist.
func (g *GhPRWriter) ApplyLabel(_ context.Context, prNumber, label string) error {
	ghClient := &GHClient{Dir: g.RepoDir}
	// Ensure label exists (--force is idempotent)
	_, _ = ghClient.runGH("label", "create", label, "--force")
	// Apply to PR
	_, err := ghClient.runGH("pr", "edit", strings.TrimPrefix(prNumber, "#"), "--add-label", label)
	if err != nil {
		return fmt.Errorf("apply label %q to PR %s: %w", label, prNumber, err)
	}
	return nil
}

// RemoveLabel removes a label from the given PR.
func (g *GhPRWriter) RemoveLabel(_ context.Context, prNumber, label string) error {
	ghClient := &GHClient{Dir: g.RepoDir}
	_, err := ghClient.runGH("pr", "edit", strings.TrimPrefix(prNumber, "#"), "--remove-label", label)
	if err != nil {
		return fmt.Errorf("remove label %q from PR %s: %w", label, prNumber, err)
	}
	return nil
}

// DeleteLabel deletes a label definition from the repository.
func (g *GhPRWriter) DeleteLabel(_ context.Context, label string) error {
	ghClient := &GHClient{Dir: g.RepoDir}
	_, err := ghClient.runGH("label", "delete", label, "--yes")
	if err != nil {
		return fmt.Errorf("delete label %q: %w", label, err)
	}
	return nil
}

// ClosePR closes the given PR with a comment.
func (g *GhPRWriter) ClosePR(_ context.Context, prNumber, comment string) error {
	ghClient := &GHClient{Dir: g.RepoDir}
	num := strings.TrimPrefix(prNumber, "#")
	_, err := ghClient.runGH("pr", "close", num, "--comment", comment)
	if err != nil {
		return fmt.Errorf("close PR %s: %w", prNumber, err)
	}
	return nil
}

// MergePR merges the given PR using the specified method.
// For squash: uses --squash --delete-branch (clean history + branch cleanup).
// For merge: uses --merge without --delete-branch (preserve hash for chain dependents).
func (g *GhPRWriter) MergePR(_ context.Context, prNumber string, method domain.MergeMethod) error {
	ghClient := &GHClient{Dir: g.RepoDir}
	num := strings.TrimPrefix(prNumber, "#")
	args := []string{"pr", "merge", num}
	switch method {
	case domain.MergeMethodSquash:
		args = append(args, "--squash", "--delete-branch")
	case domain.MergeMethodMerge:
		args = append(args, "--merge")
	default:
		return fmt.Errorf("unknown merge method: %s", method)
	}
	_, err := ghClient.runGH(args...)
	if err != nil {
		return fmt.Errorf("merge PR %s via %s: %w", prNumber, method, err)
	}
	return nil
}
