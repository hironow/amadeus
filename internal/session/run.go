package session

import (
	"context"
	"errors"
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
	if err := a.autoRebuildIfNeeded(ctx, opts.Quiet); err != nil {
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
	if err := a.Emitter.EmitRunStarted(ctx, domain.RunStartedData{
		IntegrationBranch: integrationBranch,
		BaseBranch:        opts.BaseBranch,
	}, now); err != nil {
		return fmt.Errorf("emit run started: %w", err)
	}

	// Ensure run.stopped is emitted on any post-started error exit.
	// The signal and channel_closed paths emit their own run.stopped,
	// so this defer only fires when runErr is non-nil.
	var runErr error
	defer func() {
		if runErr != nil {
			stopNow := time.Now().UTC()
			_ = a.Emitter.EmitRunStopped(ctx, domain.RunStoppedData{Reason: domain.RunStoppedReasonError}, stopNow)
		}
	}()

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
			runErr = fmt.Errorf("inbox monitor: %w", monErr)
			return runErr
		}
	}

	// Initial pre-merge check on startup (don't wait for first D-Mail)
	if a.PRPipeline != nil {
		if !opts.Quiet {
			a.Logger.Info("amadeus run: running initial PR convergence check...")
		}
		dmails, prErr := a.runPreMergePipeline(ctx, integrationBranch)
		if prErr != nil {
			a.Logger.Warn("initial pre-merge pipeline error: %v", prErr)
		} else if len(dmails) > 0 && !opts.Quiet {
			a.Logger.OK("initial check: generated %d implementation-feedback D-Mail(s)", len(dmails))
		}
	}

	// Initial PR diff review on startup (ADR-0024)
	if a.PRReader != nil {
		if !opts.Quiet {
			a.Logger.Info("amadeus run: running initial PR diff review...")
		}
		reviewDMails, reviewErr := a.evaluatePRDiffs(ctx, integrationBranch)
		if reviewErr != nil {
			a.Logger.Warn("initial PR diff review error: %v", reviewErr)
		} else if len(reviewDMails) > 0 && !opts.Quiet {
			a.Logger.OK("initial review: generated %d PR review D-Mail(s)", len(reviewDMails))
		}
	}

	// Startup auto-merge: attempt merge before entering waiting mode.
	// Guard mirrors the daemon loop's DriftError check:
	//   DriftError is returned only when D-Mails are generated (len(DMails) > 0).
	//   A non-zero Divergence with zero D-Mails means "minor drift, no action needed"
	//   and is NOT world-line divergence (世界線逸脱).
	if opts.AutoMerge && !opts.DryRun && opts.BaseBranch != "" {
		previous, loadErr := a.Store.LoadLatest()
		if loadErr != nil {
			a.Logger.Warn("startup auto-merge: load previous state: %v", loadErr)
		} else if previous.CheckedAt.IsZero() {
			if !opts.Quiet {
				a.Logger.Info("amadeus run: no prior check state, skipping startup auto-merge")
			}
		} else if len(previous.DMails) > 0 {
			a.Logger.Warn("Previous check generated %d D-Mail(s) (divergence=%.2f) — world line diverged, skipping startup auto-merge", len(previous.DMails), previous.Divergence)
		} else {
			if !opts.Quiet {
				a.Logger.Info("amadeus run: attempting startup auto-merge (last check: no D-Mails, divergence=%.2f)...", previous.Divergence)
			}
			if merged := a.attemptAutoMerge(ctx, integrationBranch); merged > 0 {
				a.closeReadyIssues(ctx, opts.ReadyLabel)
				// PRs were merged — main branch changed. Re-run pipelines to:
				// 1. Detect new conflicts and generate D-Mails for paintress
				// 2. Re-review remaining PRs (their merge status may have changed)
				a.rerunPipelinesAfterMerge(ctx, integrationBranch, opts.Quiet)
			}
		}
	}

	if !opts.Quiet {
		a.Logger.Info("Waiting for incoming D-Mails... (timeout: %s)", a.Config.IdleTimeout)
		a.Logger.Info("Press Ctrl+C to exit.")
	}

	// Main loop: event-driven via channel
	for {
		select {
		case <-ctx.Done():
			stopNow := time.Now().UTC()
			_ = a.Emitter.EmitRunStopped(ctx, domain.RunStoppedData{Reason: domain.RunStoppedReasonSignal}, stopNow)
			if !opts.Quiet {
				a.Logger.Info("amadeus run: stopped (signal)")
			}
			return nil

		case dmail, ok := <-inboxCh:
			if !ok {
				// Channel closed
				stopNow := time.Now().UTC()
				_ = a.Emitter.EmitRunStopped(ctx, domain.RunStoppedData{Reason: domain.RunStoppedReasonChannelClosed}, stopNow)
				return nil
			}

			inboxNow := time.Now().UTC()
			domain.LogBanner(a.Logger, domain.BannerRecv, string(dmail.Kind), dmail.Name, dmail.Description)
			if err := a.Emitter.EmitInboxConsumed(ctx, domain.InboxConsumedData{
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

				// Re-evaluate PR diffs (new PRs may have arrived) — ADR-0024
				if a.PRReader != nil {
					reviewDMails, reviewErr := a.evaluatePRDiffs(ctx, integrationBranch)
					if reviewErr != nil {
						a.Logger.Warn("PR diff review error: %v", reviewErr)
					} else if len(reviewDMails) > 0 && !opts.Quiet {
						a.Logger.OK("generated %d PR review D-Mail(s)", len(reviewDMails))
					}
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
						checkErr := a.runPostMergeCheck(ctx, checkOpts, []domain.DMail{dmail})
						if checkErr != nil {
							var de *domain.DriftError
							if errors.As(checkErr, &de) {
								a.Logger.Warn("World line diverged (score=%.2f) — auto-merge suspended", de.Divergence)
							} else {
								a.Logger.Warn("post-merge check error: %v", checkErr)
							}
						} else if opts.AutoMerge && !opts.DryRun {
							// No drift detected — attempt auto-merge of eligible PRs
							if merged := a.attemptAutoMerge(ctx, integrationBranch); merged > 0 {
								a.closeReadyIssues(ctx, opts.ReadyLabel)
								a.rerunPipelinesAfterMerge(ctx, integrationBranch, opts.Quiet)
							}
						}
					}
				}
			}
		}
	}
}

// rerunPipelinesAfterMerge re-runs the PR convergence and review pipelines after
// successful merges. This detects new conflicts (from merged code changing main)
// and generates D-Mails for paintress to fix them, keeping the convergence loop alive.
func (a *Amadeus) rerunPipelinesAfterMerge(ctx context.Context, integrationBranch string, quiet bool) {
	if !quiet {
		a.Logger.Info("amadeus run: re-running pipelines after merge...")
	}

	if a.PRPipeline != nil {
		dmails, prErr := a.runPreMergePipeline(ctx, integrationBranch)
		if prErr != nil {
			a.Logger.Warn("post-merge pipeline error: %v", prErr)
		} else if len(dmails) > 0 && !quiet {
			a.Logger.OK("post-merge: generated %d convergence D-Mail(s) for conflicting PRs", len(dmails))
		}
	}

	if a.PRReader != nil {
		reviewDMails, reviewErr := a.evaluatePRDiffs(ctx, integrationBranch)
		if reviewErr != nil {
			a.Logger.Warn("post-merge PR review error: %v", reviewErr)
		} else if len(reviewDMails) > 0 && !quiet {
			a.Logger.OK("post-merge: generated %d PR review D-Mail(s)", len(reviewDMails))
		}
	}
}

// runPostMergeCheck runs the existing 5-phase divergence check pipeline (Phases 1-4).
// Phase 0 (inbox) is handled by the daemon loop, so this skips it.
// inboxDMails carries the D-Mails that triggered this check (for feedback_round propagation).
func (a *Amadeus) runPostMergeCheck(ctx context.Context, opts domain.CheckOptions, inboxDMails []domain.DMail) error {
	previous, err := a.Store.LoadLatest()
	if err != nil {
		return fmt.Errorf("load previous state: %w", err)
	}

	report, fullCheck, wasForced, err := a.detectShift(ctx, previous, opts.Full, opts.Quiet)
	if err != nil {
		return err
	}

	if !report.Significant {
		if !opts.Quiet {
			a.Logger.Info("post-merge: no significant shift detected")
		}
		return nil
	}

	prompt, cleanup, err := a.buildCheckPrompt(ctx, report, fullCheck, previous, opts.Quiet)
	if err != nil {
		return fmt.Errorf("post-merge (build prompt): %w", err)
	}
	defer cleanup()

	if !opts.Quiet {
		a.Logger.Info("Divergence Meter: evaluating with %s...", a.ClaudeModel)
	}
	meterResult, err := a.runDivergenceMeter(ctx, prompt, fullCheck, previous, opts.Quiet)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	dmails, err := a.generateDMails(ctx, meterResult, inboxDMails, now)
	if err != nil {
		return err
	}

	convergenceAlerts, convergenceDMails, err := a.detectConvergence(ctx, now)
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

	a.State.AdvanceCheckCount(fullCheck, wasForced)
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

	if err := a.Emitter.EmitCheck(ctx, result, now); err != nil {
		return fmt.Errorf("emit check completed: %w", err)
	}

	if len(dmails) > 0 {
		return &domain.DriftError{Divergence: result.Divergence, DMails: len(dmails)}
	}
	return nil
}
