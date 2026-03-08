package domain

import (
	"fmt"
	"strings"
)

// BuildConvergenceDMailBody produces a Markdown body from a PRConvergenceReport.
// Pure function with no I/O.
func BuildConvergenceDMailBody(report PRConvergenceReport) string {
	var sb strings.Builder

	sb.WriteString("## PR Dependency Chain Analysis\n\n")
	sb.WriteString(fmt.Sprintf("Integration branch: `%s` | Total open PRs: %d\n\n", report.IntegrationBranch, report.TotalOpenPRs))

	for _, chain := range report.Chains {
		sb.WriteString(fmt.Sprintf("### %s\n\n", chain.ID))

		// Chain structure visualization.
		sb.WriteString("**Chain structure:** ")
		for i, pr := range chain.PRs {
			if i == 0 {
				sb.WriteString(fmt.Sprintf("%s (base: %s)", pr.Number(), pr.BaseBranch()))
			} else {
				sb.WriteString(fmt.Sprintf(" <- %s (base: %s)", pr.Number(), pr.BaseBranch()))
			}
		}
		sb.WriteString("\n\n")

		// Issue table.
		sb.WriteString("| PR | Base | Status | Issue |\n")
		sb.WriteString("|---|---|---|---|\n")
		for _, pr := range chain.PRs {
			status := "mergeable"
			issue := "-"
			if pr.HasConflict() {
				status = "conflict"
				issue = fmt.Sprintf("conflicts in: %s", strings.Join(pr.ConflictFiles(), ", "))
			} else if pr.BehindBy() > 0 {
				status = fmt.Sprintf("behind by %d", pr.BehindBy())
				issue = "needs rebase"
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", pr.Number(), pr.BaseBranch(), status, issue))
		}
		sb.WriteString("\n")

		// Recommended actions.
		sb.WriteString("**Recommended merge order:** ")
		for i, pr := range chain.PRs {
			if i > 0 {
				sb.WriteString(" -> ")
			}
			sb.WriteString(pr.Number())
		}
		sb.WriteString(" (root first, then dependents)\n\n")
	}

	// Conflict details section if any chain has conflicts.
	hasAnyConflict := false
	for _, chain := range report.Chains {
		if chain.HasConflict {
			hasAnyConflict = true
			break
		}
	}
	if hasAnyConflict {
		sb.WriteString("### Conflict Details\n\n")
		for _, chain := range report.Chains {
			if !chain.HasConflict {
				continue
			}
			for _, pr := range chain.PRs {
				if !pr.HasConflict() {
					continue
				}
				sb.WriteString(fmt.Sprintf("- %s: %s\n", pr.Number(), strings.Join(pr.ConflictFiles(), ", ")))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// BuildConvergenceDMail constructs a valid DMail from a PRConvergenceReport.
// Uses KindImplFeedback and ClassifyConvergenceScenario for the most severe chain.
func BuildConvergenceDMail(name string, report PRConvergenceReport) DMail {
	// Determine worst severity across all chains.
	worstSeverity := SeverityLow
	worstAction := ActionResolve
	for _, chain := range report.Chains {
		sev, act := ClassifyConvergenceScenario(chain)
		if severityRank(sev) > severityRank(worstSeverity) {
			worstSeverity = sev
			worstAction = act
		}
	}

	// Collect all PR numbers as targets.
	var targets []string
	for _, chain := range report.Chains {
		for _, pr := range chain.PRs {
			targets = append(targets, pr.Number())
		}
	}
	for _, pr := range report.OrphanedPRs {
		targets = append(targets, pr.Number())
	}

	// Build description summarizing chains.
	desc := buildConvergenceDescription(report)

	// Conflict PRs for metadata.
	var conflictPRs []string
	for _, chain := range report.Chains {
		for _, pr := range chain.PRs {
			if pr.HasConflict() {
				conflictPRs = append(conflictPRs, pr.Number())
			}
		}
	}

	metadata := map[string]string{
		"integration_branch": report.IntegrationBranch,
		"chain_count":        fmt.Sprintf("%d", len(report.Chains)),
		"conflict_prs":       strings.Join(conflictPRs, ","),
	}

	return DMail{
		SchemaVersion: DMailSchemaVersion,
		Name:          name,
		Kind:          KindImplFeedback,
		Description:   desc,
		Severity:      worstSeverity,
		Action:        worstAction,
		Targets:       targets,
		Metadata:      metadata,
		Body:          BuildConvergenceDMailBody(report),
	}
}

// buildConvergenceDescription summarizes chains for the D-Mail description field.
func buildConvergenceDescription(report PRConvergenceReport) string {
	if len(report.Chains) == 0 {
		return "No PR dependency chains detected"
	}
	// Summarize the first chain as example.
	chain := report.Chains[0]
	var nums []string
	for _, pr := range chain.PRs {
		nums = append(nums, pr.Number())
	}
	desc := fmt.Sprintf("PR dependency chain requires convergence: %s", strings.Join(nums, " -> "))
	if len(report.Chains) > 1 {
		desc += fmt.Sprintf(" (+%d more chains)", len(report.Chains)-1)
	}
	return desc
}

// severityRank returns a numeric rank for severity comparison.
func severityRank(s Severity) int {
	switch s {
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	default:
		return 0
	}
}
