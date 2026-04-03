package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// prPipelineRunner implements port.PRPipelineRunner using pure usecase logic.
type prPipelineRunner struct {
	prReader port.GitHubPRReader
	store    port.StateReader
	emitter  port.CheckEventEmitter
	logger   domain.Logger
}

// NewPRPipelineRunner creates a PRPipelineRunner with the given dependencies.
func NewPRPipelineRunner(prReader port.GitHubPRReader, store port.StateReader,
	emitter port.CheckEventEmitter, logger domain.Logger,
) port.PRPipelineRunner {
	return &prPipelineRunner{
		prReader: prReader,
		store:    store,
		emitter:  emitter,
		logger:   logger,
	}
}

// RunPreMergePipeline analyzes open PRs for dependency chains and conflicts,
// then generates implementation-feedback D-Mails for chains needing action.
// Returns nil when prReader is nil (PR convergence disabled) or no open PRs.
func (r *prPipelineRunner) RunPreMergePipeline(ctx context.Context, integrationBranch string) ([]domain.DMail, error) {
	return runPreMergePipeline(ctx, integrationBranch, r.prReader, r.store, r.emitter, r.logger)
}

// runPreMergePipeline is the core logic, testable with explicit dependencies.
func runPreMergePipeline(ctx context.Context, integrationBranch string,
	prReader port.GitHubPRReader, store port.StateReader,
	emitter port.CheckEventEmitter, logger domain.Logger,
) ([]domain.DMail, error) {
	if prReader == nil {
		return nil, nil
	}

	logger.Info("PR convergence: fetching open PRs...")
	prs, err := prReader.ListOpenPRs(ctx, integrationBranch)
	if err != nil {
		return nil, fmt.Errorf("list open PRs: %w", err)
	}

	if len(prs) == 0 {
		logger.Info("PR convergence: no open PRs")
		return nil, nil
	}
	logger.Info("PR convergence: fetched %d open PRs, analyzing chains...", len(prs))

	report := harness.BuildPRConvergenceReport(integrationBranch, prs)

	if len(report.Chains) == 0 && len(report.OrphanedPRs) == 0 {
		logger.Info("PR convergence: %d open PRs, no chains or orphans detected", report.TotalOpenPRs)
		return nil, nil
	}

	logger.Info("PR convergence: %d open PRs, %d chain(s), %d orphaned",
		report.TotalOpenPRs, len(report.Chains), len(report.OrphanedPRs))

	now := time.Now().UTC()
	var dmails []domain.DMail
	var conflictCount int

	for _, chain := range report.Chains {
		needsAction := len(chain.PRs) > 1 || chain.HasConflict
		if !needsAction {
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

		name, nameErr := store.NextDMailName(domain.KindImplFeedback)
		if nameErr != nil {
			return dmails, fmt.Errorf("pr convergence (dmail name): %w", nameErr)
		}

		singleChainReport := domain.PRConvergenceReport{
			IntegrationBranch: integrationBranch,
			Chains:            []domain.PRChain{chain},
			TotalOpenPRs:      report.TotalOpenPRs,
		}
		dmail := harness.BuildConvergenceDMail(name, singleChainReport)

		if errs := harness.ValidateDMail(dmail); len(errs) > 0 {
			logger.Warn("skipping invalid PR convergence dmail %s: %v", name, errs)
			continue
		}

		domain.LogBanner(logger, domain.BannerSend, string(dmail.Kind), dmail.Name, dmail.Description)
		if err := emitter.EmitDMailGenerated(dmail, now); err != nil {
			return dmails, fmt.Errorf("pr convergence (emit dmail): %w", err)
		}
		dmails = append(dmails, dmail)
	}

	if err := emitter.EmitPRConvergenceChecked(domain.PRConvergenceCheckedData{
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
