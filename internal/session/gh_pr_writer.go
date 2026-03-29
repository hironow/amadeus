package session

import (
	"context"
	"fmt"
	"strings"

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
