package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness/policy"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// Compile-time check that GhPRReader implements port.GitHubPRReader.
var _ port.GitHubPRReader = (*GhPRReader)(nil)

// GhPRReader implements GitHubPRReader using the gh CLI.
type GhPRReader struct {
	RepoDir string
}

// NewGhPRReader creates a new GhPRReader for the given repository directory.
func NewGhPRReader(repoDir string) *GhPRReader {
	return &GhPRReader{RepoDir: repoDir}
}

// ghPRListEntry is the JSON structure returned by `gh pr list --json`.
type ghPRListEntry struct {
	Number      int            `json:"number"`
	Title       string         `json:"title"`
	BaseRefName string         `json:"baseRefName"`
	HeadRefName string         `json:"headRefName"`
	Mergeable   string         `json:"mergeable"` // "MERGEABLE", "CONFLICTING", "UNKNOWN"
	Labels      []ghLabelEntry `json:"labels"`
	HeadRefOid  string         `json:"headRefOid"`
}

// ghLabelEntry is the JSON structure for a label in `gh pr list --json`.
type ghLabelEntry struct {
	Name string `json:"name"`
}

// ListOpenPRs returns all open PRs targeting the given branch.
func (g *GhPRReader) ListOpenPRs(_ context.Context, targetBranch string) ([]domain.PRState, error) {
	ghClient := &GHClient{Dir: g.RepoDir}
	args := []string{
		"pr", "list",
		"--state", "open",
		"--json", "number,title,baseRefName,headRefName,mergeable,labels,headRefOid",
		"--limit", "100",
	}
	if targetBranch != "" {
		args = append(args, "--base", targetBranch)
	}
	data, err := ghClient.runGH(args...)
	if err != nil {
		return nil, fmt.Errorf("list open PRs: %w", err)
	}
	return parseGhPRListOutput(data)
}

// GetPRDiff returns the unified diff for the given PR number.
func (g *GhPRReader) GetPRDiff(_ context.Context, prNumber string) (string, error) {
	ghClient := &GHClient{Dir: g.RepoDir}
	data, err := ghClient.runGH("pr", "diff", strings.TrimPrefix(prNumber, "#"))
	if err != nil {
		return "", fmt.Errorf("get PR diff for %s: %w", prNumber, err)
	}
	return string(data), nil
}

// ghPRViewEntry is the JSON structure returned by `gh pr view --json` for merge readiness.
type ghPRViewEntry struct {
	MergeStateStatus string         `json:"mergeStateStatus"` // "CLEAN", "BLOCKED", "BEHIND", "DIRTY", "UNSTABLE"
	ReviewDecision   string         `json:"reviewDecision"`   // "APPROVED", "REVIEW_REQUIRED", "CHANGES_REQUESTED", ""
	Mergeable        string         `json:"mergeable"`        // "MERGEABLE", "CONFLICTING", "UNKNOWN"
	Labels           []ghLabelEntry `json:"labels"`
	HeadRefOid       string         `json:"headRefOid"`
}

// GetPRMergeReadiness returns the merge readiness state for the given PR number.
func (g *GhPRReader) GetPRMergeReadiness(_ context.Context, prNumber string) (*domain.PRMergeReadiness, error) {
	ghClient := &GHClient{Dir: g.RepoDir}
	data, err := ghClient.runGH(
		"pr", "view", strings.TrimPrefix(prNumber, "#"),
		"--json", "mergeStateStatus,reviewDecision,mergeable,labels,headRefOid",
	)
	if err != nil {
		return nil, fmt.Errorf("get PR merge readiness for %s: %w", prNumber, err)
	}

	var entry ghPRViewEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("parse PR view JSON for %s: %w", prNumber, err)
	}

	hasReviewLabel := false
	for _, l := range entry.Labels {
		// Accept both new ("amadeus:reviewed") and legacy ("amadeus:reviewed-{sha8}") formats
		if l.Name == PRReviewLabel || strings.HasPrefix(l.Name, PRReviewLabelLegacyPrefix) {
			hasReviewLabel = true
			break
		}
	}

	r := policy.EvaluateMergeReadiness(
		prNumber,
		entry.MergeStateStatus,
		entry.ReviewDecision,
		entry.Mergeable,
		hasReviewLabel,
	)
	return &r, nil
}

// parseGhPRListOutput parses the JSON output from `gh pr list --json` into domain PRState slice.
func parseGhPRListOutput(data []byte) ([]domain.PRState, error) {
	var entries []ghPRListEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse gh pr list JSON: %w", err)
	}

	prs := make([]domain.PRState, 0, len(entries))
	for _, e := range entries {
		number := "#" + strconv.Itoa(e.Number)
		mergeable := e.Mergeable == "MERGEABLE"

		var labels []string
		for _, l := range e.Labels {
			labels = append(labels, l.Name)
		}

		ps, err := domain.NewPRState(number, e.Title, e.BaseRefName, e.HeadRefName, mergeable, 0, nil, labels, e.HeadRefOid)
		if err != nil {
			return nil, fmt.Errorf("construct PRState for %s: %w", number, err)
		}
		prs = append(prs, ps)
	}
	return prs, nil
}
