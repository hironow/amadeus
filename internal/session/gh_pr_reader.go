package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hironow/amadeus/internal/domain"
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
	Number      int    `json:"number"`
	Title       string `json:"title"`
	BaseRefName string `json:"baseRefName"`
	HeadRefName string `json:"headRefName"`
	Mergeable   string `json:"mergeable"` // "MERGEABLE", "CONFLICTING", "UNKNOWN"
}

// ListOpenPRs returns all open PRs. The caller handles chain building and filtering.
func (g *GhPRReader) ListOpenPRs(ctx context.Context, _ string) ([]domain.PRState, error) {
	ghClient := &GHClient{Dir: g.RepoDir}
	data, err := ghClient.runGH(
		"pr", "list",
		"--state", "open",
		"--json", "number,title,baseRefName,headRefName,mergeable",
		"--limit", "100",
	)
	if err != nil {
		return nil, fmt.Errorf("list open PRs: %w", err)
	}
	return parseGhPRListOutput(data)
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

		ps, err := domain.NewPRState(number, e.Title, e.BaseRefName, e.HeadRefName, mergeable, 0, nil)
		if err != nil {
			return nil, fmt.Errorf("construct PRState for %s: %w", number, err)
		}
		prs = append(prs, ps)
	}
	return prs, nil
}
