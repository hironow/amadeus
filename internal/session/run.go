package session

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// Run executes the inbox-driven daemon loop.
// It scans the inbox for D-Mails, triggers pre-merge analysis on report D-Mails,
// and optionally runs post-merge divergence checks when BaseBranch is set.
// The loop exits when ctx is cancelled.
func (a *Amadeus) Run(ctx context.Context, opts domain.RunOptions, emitter port.CheckEventEmitter, state port.CheckStateProvider) error {
	if emitter != nil {
		a.Emitter = emitter
	}
	if state != nil {
		a.State = state
	}

	ctx, span := platform.Tracer.Start(ctx, "amadeus.run")
	defer span.End()

	// Auto-rebuild projections if needed
	if err := a.autoRebuildIfNeeded(opts.Quiet); err != nil {
		return fmt.Errorf("auto-rebuild: %w", err)
	}

	// Determine integration branch: --base flag takes precedence, then
	// current branch, then "main" as last resort. PR convergence builds
	// chains from PRs whose baseBranch matches this value, so it must be
	// the branch that PRs target (typically "main").
	integrationBranch := opts.BaseBranch
	if integrationBranch == "" {
		var err error
		integrationBranch, err = a.Git.CurrentBranch()
		if err != nil {
			integrationBranch = "main"
			a.Logger.Warn("could not detect current branch, using %q", integrationBranch)
		}
	}

	// Emit run.started
	now := time.Now().UTC()
	if err := a.Emitter.EmitRunStarted(domain.RunStartedData{
		IntegrationBranch: integrationBranch,
		BaseBranch:        opts.BaseBranch,
	}, now); err != nil {
		return fmt.Errorf("emit run started: %w", err)
	}

	if !opts.Quiet {
		a.Logger.Info("amadeus run: integration point = %s", integrationBranch)
		if opts.BaseBranch != "" {
			a.Logger.Info("amadeus run: post-merge checks enabled (base = %s)", opts.BaseBranch)
		}
	}

	// Start inbox monitor
	var inboxCh <-chan domain.DMail
	if a.InboxCh != nil {
		inboxCh = a.InboxCh
	} else {
		stateDir := filepath.Join(a.RepoDir, domain.StateDir)
		var monErr error
		inboxCh, monErr = MonitorInbox(ctx, stateDir, a.Logger)
		if monErr != nil {
			return fmt.Errorf("inbox monitor: %w", monErr)
		}
	}

	// Main loop: event-driven via channel
	for {
		select {
		case <-ctx.Done():
			stopNow := time.Now().UTC()
			_ = a.Emitter.EmitRunStopped(domain.RunStoppedData{Reason: "signal"}, stopNow)
			if !opts.Quiet {
				a.Logger.Info("amadeus run: stopped (signal)")
			}
			return nil

		case dmail, ok := <-inboxCh:
			if !ok {
				// Channel closed
				stopNow := time.Now().UTC()
				_ = a.Emitter.EmitRunStopped(domain.RunStoppedData{Reason: "channel_closed"}, stopNow)
				return nil
			}

			inboxNow := time.Now().UTC()
			domain.LogBanner(a.Logger, domain.BannerRecv, string(dmail.Kind), dmail.Name, dmail.Description)
			if err := a.Emitter.EmitInboxConsumed(domain.InboxConsumedData{
				Name:   dmail.Name,
				Kind:   dmail.Kind,
				Source: dmail.Name + ".md",
			}, inboxNow); err != nil {
				a.Logger.Warn("emit inbox consumed: %v", err)
			}

			if !opts.Quiet {
				a.Logger.Info("consumed D-Mail from inbox: %s", dmail.Name)
			}

			if dmail.Kind == domain.KindReport {
				dmails, prErr := a.runPreMergePipeline(ctx, integrationBranch)
				if prErr != nil {
					a.Logger.Warn("pre-merge pipeline error: %v", prErr)
				} else if len(dmails) > 0 && !opts.Quiet {
					a.Logger.OK("generated %d implementation-feedback D-Mail(s)", len(dmails))
				}

				if opts.BaseBranch != "" {
					previous, loadErr := a.Store.LoadLatest()
					if loadErr != nil {
						a.Logger.Warn("load previous state: %v", loadErr)
					} else {
						a.State.Restore(previous)
						checkOpts := domain.CheckOptions{
							Full:  opts.Full,
							Quiet: opts.Quiet,
							JSON:  opts.JSON,
						}
						if checkErr := a.runPostMergeCheck(ctx, checkOpts); checkErr != nil {
							if _, ok := checkErr.(*domain.DriftError); !ok {
								a.Logger.Warn("post-merge check error: %v", checkErr)
							}
						}
					}
				}
			}
		}
	}
}

// runPostMergeCheck runs the existing 5-phase divergence check pipeline (Phases 1-4).
// Phase 0 (inbox) is handled by the daemon loop, so this skips it.
func (a *Amadeus) runPostMergeCheck(ctx context.Context, opts domain.CheckOptions) error {
	previous, err := a.Store.LoadLatest()
	if err != nil {
		return fmt.Errorf("load previous state: %w", err)
	}

	report, fullCheck, err := a.detectShift(ctx, previous, opts.Full, opts.Quiet)
	if err != nil {
		return err
	}

	if !report.Significant {
		if !opts.Quiet {
			a.Logger.Info("post-merge: no significant shift detected")
		}
		return nil
	}

	prompt, err := a.buildCheckPrompt(ctx, report, fullCheck, previous, opts.Quiet)
	if err != nil {
		return fmt.Errorf("post-merge (build prompt): %w", err)
	}

	meterResult, err := a.runDivergenceMeter(ctx, prompt, fullCheck, previous, opts.Quiet)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	dmails, err := a.generateDMails(ctx, meterResult, now)
	if err != nil {
		return err
	}

	convergenceAlerts, convergenceDMails, err := a.detectConvergence(now)
	if err != nil {
		return err
	}
	dmails = append(dmails, convergenceDMails...)

	currentCommit, commitErr := a.Git.CurrentCommit()
	if commitErr != nil {
		return fmt.Errorf("get current commit: %w", commitErr)
	}

	var prNumbers []string
	for _, pr := range report.MergedPRs {
		prNumbers = append(prNumbers, pr.Number)
	}
	var dmailNames []string
	for _, d := range dmails {
		dmailNames = append(dmailNames, d.Name)
	}

	a.State.AdvanceCheckCount(fullCheck)
	checkType := domain.CheckTypeDiff
	if fullCheck {
		checkType = domain.CheckTypeFull
	}

	result := domain.CheckResult{
		CheckedAt:         now,
		Commit:            currentCommit,
		Type:              checkType,
		Divergence:        meterResult.Divergence.Value,
		Axes:              meterResult.Divergence.Axes,
		ImpactRadius:      meterResult.ImpactRadius,
		PRsEvaluated:      prNumbers,
		DMails:            dmailNames,
		ConvergenceAlerts: convergenceAlerts,
	}

	if err := a.Emitter.EmitCheck(result, now); err != nil {
		return fmt.Errorf("emit check completed: %w", err)
	}

	if len(dmails) > 0 {
		return &domain.DriftError{Divergence: result.Divergence, DMails: len(dmails)}
	}
	return nil
}
