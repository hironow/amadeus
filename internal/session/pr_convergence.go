package session

import (
	"context"
	"fmt"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// runPreMergePipeline analyzes open PRs for dependency chains and conflicts,
// then generates implementation-feedback D-Mails for chains needing action.
func (a *Amadeus) runPreMergePipeline(ctx context.Context, integrationBranch string) ([]domain.DMail, error) {
	if a.PRReader == nil {
		return nil, nil // PR convergence disabled
	}

	_, span := platform.Tracer.Start(ctx, "pr_convergence",
		trace.WithAttributes(
			attribute.String("integration_branch", integrationBranch),
		))
	defer span.End()

	// 1. List all open PRs via gh CLI
	prs, err := a.PRReader.ListOpenPRs(ctx, integrationBranch)
	if err != nil {
		return nil, fmt.Errorf("list open PRs: %w", err)
	}

	if len(prs) == 0 {
		a.Logger.Info("PR convergence: no open PRs")
		return nil, nil
	}

	// 2. Build convergence report (pure domain)
	report := domain.BuildPRConvergenceReport(integrationBranch, prs)

	if len(report.Chains) == 0 && len(report.OrphanedPRs) == 0 {
		a.Logger.Info("PR convergence: %d open PRs, no chains or orphans detected", report.TotalOpenPRs)
		return nil, nil
	}

	a.Logger.Info("PR convergence: %d open PRs, %d chain(s), %d orphaned",
		report.TotalOpenPRs, len(report.Chains), len(report.OrphanedPRs))

	// 3. Generate D-Mails for chains needing action
	now := time.Now().UTC()
	var dmails []domain.DMail
	var conflictCount int

	for _, chain := range report.Chains {
		// Skip chains with no issues (single PR, mergeable, not behind)
		needsAction := len(chain.PRs) > 1 || chain.HasConflict
		if !needsAction {
			// Single PR with no conflict — check if behind
			if len(chain.PRs) == 1 && chain.PRs[0].BehindBy() > 0 {
				needsAction = true
			}
		}
		if !needsAction {
			continue
		}

		if chain.HasConflict {
			conflictCount++
		}

		// Build D-Mail for this chain
		name, nameErr := a.Store.NextDMailName(domain.KindImplFeedback)
		if nameErr != nil {
			return dmails, fmt.Errorf("pr convergence (dmail name): %w", nameErr)
		}

		// Build a single-chain report for D-Mail body
		singleChainReport := domain.PRConvergenceReport{
			IntegrationBranch: integrationBranch,
			Chains:            []domain.PRChain{chain},
			TotalOpenPRs:      report.TotalOpenPRs,
		}
		dmail := domain.BuildConvergenceDMail(name, singleChainReport)

		if errs := domain.ValidateDMail(dmail); len(errs) > 0 {
			a.Logger.Warn("skipping invalid PR convergence dmail %s: %v", name, errs)
			continue
		}

		domain.LogBanner(a.Logger, domain.BannerSend, string(dmail.Kind), dmail.Name, dmail.Description)
		if err := a.Emitter.EmitDMailGenerated(dmail, now); err != nil {
			return dmails, fmt.Errorf("pr convergence (emit dmail): %w", err)
		}
		dmails = append(dmails, dmail)
	}

	// 4. Emit pr_convergence.checked event
	if err := a.Emitter.EmitPRConvergenceChecked(domain.PRConvergenceCheckedData{
		IntegrationBranch: integrationBranch,
		TotalOpenPRs:      report.TotalOpenPRs,
		Chains:            len(report.Chains),
		ConflictPRs:       conflictCount,
		DMails:            len(dmails),
	}, now); err != nil {
		return dmails, fmt.Errorf("emit pr convergence checked: %w", err)
	}

	return dmails, nil
}
