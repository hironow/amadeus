package domain

import (
	"fmt"
	"strings"
)

// PRState represents an open PR's convergence-relevant state.
// All fields are unexported; use NewPRState to construct with validation.
type PRState struct {
	number        string
	title         string
	baseBranch    string
	headBranch    string
	mergeable     bool
	behindBy      int
	conflictFiles []string
	labels        []string
	headSHA       string
}

// NewPRState creates a validated PRState. Returns an error if required fields
// (number, baseBranch, headBranch) are empty.
func NewPRState(number, title, baseBranch, headBranch string, mergeable bool, behindBy int, conflictFiles, labels []string, headSHA string) (PRState, error) {
	if number == "" {
		return PRState{}, fmt.Errorf("PRState number is required")
	}
	if baseBranch == "" {
		return PRState{}, fmt.Errorf("PRState baseBranch is required")
	}
	if headBranch == "" {
		return PRState{}, fmt.Errorf("PRState headBranch is required")
	}
	// Defensive copy of conflictFiles to prevent mutation.
	var files []string
	if len(conflictFiles) > 0 {
		files = make([]string, len(conflictFiles))
		copy(files, conflictFiles)
	}
	// Defensive copy of labels.
	var lbls []string
	if len(labels) > 0 {
		lbls = make([]string, len(labels))
		copy(lbls, labels)
	}
	return PRState{
		number:        number,
		title:         title,
		baseBranch:    baseBranch,
		headBranch:    headBranch,
		mergeable:     mergeable,
		behindBy:      behindBy,
		conflictFiles: files,
		labels:        lbls,
		headSHA:       headSHA,
	}, nil
}

// Number returns the PR number (e.g. "#42").
func (p PRState) Number() string { return p.number }

// Title returns the PR title.
func (p PRState) Title() string { return p.title }

// BaseBranch returns the base branch of the PR.
func (p PRState) BaseBranch() string { return p.baseBranch }

// HeadBranch returns the head branch of the PR.
func (p PRState) HeadBranch() string { return p.headBranch }

// Mergeable returns whether the PR is currently mergeable.
func (p PRState) Mergeable() bool { return p.mergeable }

// BehindBy returns how many commits the PR is behind the base branch.
func (p PRState) BehindBy() int { return p.behindBy }

// ConflictFiles returns the list of files with merge conflicts.
func (p PRState) ConflictFiles() []string { return p.conflictFiles }

// HasConflict reports whether the PR has any merge conflict files.
func (p PRState) HasConflict() bool { return len(p.conflictFiles) > 0 }

// Labels returns the labels applied to the PR.
func (p PRState) Labels() []string { return p.labels }

// HasLabel reports whether the PR has the given label (exact match).
func (p PRState) HasLabel(label string) bool {
	for _, l := range p.labels {
		if l == label {
			return true
		}
	}
	return false
}

// HasLabelPrefix reports whether the PR has any label starting with the given prefix.
func (p PRState) HasLabelPrefix(prefix string) bool {
	for _, l := range p.labels {
		if strings.HasPrefix(l, prefix) {
			return true
		}
	}
	return false
}

// HeadSHA returns the full HEAD commit SHA of the PR.
func (p PRState) HeadSHA() string { return p.headSHA }

// HeadSHAShort returns the first 8 characters of the HEAD commit SHA.
func (p PRState) HeadSHAShort() string {
	if len(p.headSHA) >= 8 {
		return p.headSHA[:8]
	}
	return p.headSHA
}

// PRChain represents a dependency chain of PRs ordered root to leaf.
type PRChain struct {
	ID          string
	PRs         []PRState
	HasConflict bool
}

// PRConvergenceReport is the result of pre-merge convergence analysis.
type PRConvergenceReport struct {
	IntegrationBranch string
	Chains            []PRChain
	OrphanedPRs       []PRState
	TotalOpenPRs      int
}

// BuildPRConvergenceReport builds a convergence report from open PRs.
// Pure function: builds adjacency from baseBranch -> []PRState, walks from
// integrationBranch to find chains, identifies orphaned PRs.
func BuildPRConvergenceReport(integrationBranch string, prs []PRState) PRConvergenceReport {
	report := PRConvergenceReport{
		IntegrationBranch: integrationBranch,
		TotalOpenPRs:      len(prs),
	}
	if len(prs) == 0 {
		return report
	}

	// Build adjacency: baseBranch -> []PRState
	adjacency := make(map[string][]PRState)
	for _, pr := range prs {
		adjacency[pr.baseBranch] = append(adjacency[pr.baseBranch], pr)
	}

	// Collect all head branches that are reachable from integration branch.
	reachable := make(map[string]bool)

	// Each PR whose base is the integration branch starts a new chain.
	roots := adjacency[integrationBranch]
	chainIdx := 0
	for _, root := range roots {
		chain := buildChainBFS(root, adjacency)
		chain.ID = fmt.Sprintf("chain-%c", 'a'+rune(chainIdx))
		chainIdx++

		// Mark all PRs in this chain as reachable.
		for _, pr := range chain.PRs {
			reachable[pr.headBranch] = true
		}

		report.Chains = append(report.Chains, chain)
	}

	// Any PR not in a chain is orphaned: its baseBranch is not the integration
	// branch AND not any chain PR's head branch.
	for _, pr := range prs {
		if pr.baseBranch == integrationBranch {
			continue
		}
		if reachable[pr.baseBranch] {
			continue
		}
		report.OrphanedPRs = append(report.OrphanedPRs, pr)
	}

	return report
}

// buildChainBFS walks the adjacency map breadth-first from root, collecting
// all connected PRs into a single chain. BFS guarantees root-to-leaf ordering:
// a parent PR always appears before any of its dependents.
func buildChainBFS(root PRState, adjacency map[string][]PRState) PRChain {
	chain := PRChain{}
	visited := make(map[string]bool)
	queue := []PRState{root}
	visited[root.headBranch] = true
	for len(queue) > 0 {
		pr := queue[0]
		queue = queue[1:]
		chain.PRs = append(chain.PRs, pr)
		if pr.HasConflict() {
			chain.HasConflict = true
		}
		// Follow children: PRs whose baseBranch == this PR's headBranch.
		for _, child := range adjacency[pr.headBranch] {
			if !visited[child.headBranch] {
				visited[child.headBranch] = true
				queue = append(queue, child)
			}
		}
	}
	return chain
}

// ClassifyConvergenceScenario classifies a chain's convergence scenario.
// Returns severity and recommended DMailAction.
//   - Single PR, no conflict, behind > 0: low, retry
//   - Chain (>1 PR), no conflict: medium, retry
//   - Any conflict in chain: high, retry
func ClassifyConvergenceScenario(chain PRChain) (Severity, DMailAction) {
	if chain.HasConflict {
		return SeverityHigh, ActionRetry
	}
	if len(chain.PRs) > 1 {
		return SeverityMedium, ActionRetry
	}
	return SeverityLow, ActionRetry
}
