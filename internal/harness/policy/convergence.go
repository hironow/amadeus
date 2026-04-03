package policy

import (
	"fmt"

	"github.com/hironow/amadeus/internal/domain"
)

// BuildPRConvergenceReport builds a convergence report from open PRs.
// Pure function: builds adjacency from baseBranch -> []PRState, walks from
// integrationBranch to find chains, identifies orphaned PRs.
func BuildPRConvergenceReport(integrationBranch string, prs []domain.PRState) domain.PRConvergenceReport {
	report := domain.PRConvergenceReport{
		IntegrationBranch: integrationBranch,
		TotalOpenPRs:      len(prs),
	}
	if len(prs) == 0 {
		return report
	}

	adjacency := make(map[string][]domain.PRState)
	for _, pr := range prs {
		adjacency[pr.BaseBranch()] = append(adjacency[pr.BaseBranch()], pr)
	}

	reachable := make(map[string]bool)

	roots := adjacency[integrationBranch]
	chainIdx := 0
	for _, root := range roots {
		chain := buildChainBFS(root, adjacency)
		chain.ID = fmt.Sprintf("chain-%c", 'a'+rune(chainIdx))
		chainIdx++

		for _, pr := range chain.PRs {
			reachable[pr.HeadBranch()] = true
		}

		report.Chains = append(report.Chains, chain)
	}

	for _, pr := range prs {
		if pr.BaseBranch() == integrationBranch {
			continue
		}
		if reachable[pr.BaseBranch()] {
			continue
		}
		report.OrphanedPRs = append(report.OrphanedPRs, pr)
	}

	return report
}

// buildChainBFS walks the adjacency map breadth-first from root, collecting
// all connected PRs into a single chain.
func buildChainBFS(root domain.PRState, adjacency map[string][]domain.PRState) domain.PRChain {
	chain := domain.PRChain{}
	visited := make(map[string]bool)
	queue := []domain.PRState{root}
	visited[root.HeadBranch()] = true
	for len(queue) > 0 {
		pr := queue[0]
		queue = queue[1:]
		chain.PRs = append(chain.PRs, pr)
		if pr.HasConflict() {
			chain.HasConflict = true
		}
		for _, child := range adjacency[pr.HeadBranch()] {
			if !visited[child.HeadBranch()] {
				visited[child.HeadBranch()] = true
				queue = append(queue, child)
			}
		}
	}
	return chain
}

// ClassifyConvergenceScenario classifies a chain's convergence scenario.
func ClassifyConvergenceScenario(chain domain.PRChain) (domain.Severity, domain.DMailAction) {
	if chain.HasConflict {
		return domain.SeverityHigh, domain.ActionRetry
	}
	if len(chain.PRs) > 1 {
		return domain.SeverityMedium, domain.ActionRetry
	}
	return domain.SeverityLow, domain.ActionRetry
}
