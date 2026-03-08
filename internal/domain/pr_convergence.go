package domain

import "fmt"

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
}

// NewPRState creates a validated PRState. Returns an error if required fields
// (number, baseBranch, headBranch) are empty.
func NewPRState(number, title, baseBranch, headBranch string, mergeable bool, behindBy int, conflictFiles []string) (PRState, error) {
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
	return PRState{
		number:        number,
		title:         title,
		baseBranch:    baseBranch,
		headBranch:    headBranch,
		mergeable:     mergeable,
		behindBy:      behindBy,
		conflictFiles: files,
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
		chain := buildChainDFS(root, adjacency)
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

// buildChainDFS walks the adjacency map depth-first from root, collecting
// all connected PRs into a single chain.
func buildChainDFS(root PRState, adjacency map[string][]PRState) PRChain {
	chain := PRChain{}
	stack := []PRState{root}
	for len(stack) > 0 {
		pr := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		chain.PRs = append(chain.PRs, pr)
		if pr.HasConflict() {
			chain.HasConflict = true
		}
		// Follow children: PRs whose baseBranch == this PR's headBranch.
		children := adjacency[pr.headBranch]
		for _, child := range children {
			stack = append(stack, child)
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
