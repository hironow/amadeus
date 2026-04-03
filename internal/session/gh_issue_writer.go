package session

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hironow/amadeus/internal/usecase/port"
)

// Compile-time check that GhIssueWriter implements port.GitHubIssueWriter.
var _ port.GitHubIssueWriter = (*GhIssueWriter)(nil)

// GhIssueWriter implements GitHubIssueWriter using the gh CLI.
type GhIssueWriter struct {
	RepoDir string
}

// NewGhIssueWriter creates a new GhIssueWriter for the given repository directory.
func NewGhIssueWriter(repoDir string) *GhIssueWriter {
	return &GhIssueWriter{RepoDir: repoDir}
}

// ListOpenIssuesByLabel returns issue numbers with the given label that are still open.
func (g *GhIssueWriter) ListOpenIssuesByLabel(_ context.Context, label string) ([]string, error) {
	ghClient := &GHClient{Dir: g.RepoDir}
	out, err := ghClient.runGH("issue", "list", "--state", "open", "--label", label, "--json", "number", "--limit", "100")
	if err != nil {
		return nil, fmt.Errorf("list issues by label %s: %w", label, err)
	}
	var items []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parse issue list: %w", err)
	}
	numbers := make([]string, 0, len(items))
	for _, item := range items {
		numbers = append(numbers, fmt.Sprintf("%d", item.Number))
	}
	return numbers, nil
}

// CloseIssue closes the given issue with a comment.
func (g *GhIssueWriter) CloseIssue(_ context.Context, issueNumber, comment string) error {
	ghClient := &GHClient{Dir: g.RepoDir}
	_, err := ghClient.runGH("issue", "close", issueNumber, "--comment", comment)
	if err != nil {
		return fmt.Errorf("close issue %s: %w", issueNumber, err)
	}
	return nil
}
