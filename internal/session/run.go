package session

import (
	"context"
	"fmt"
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

	// Determine integration branch
	integrationBranch, err := a.Git.CurrentBranch()
	if err != nil {
		// Fallback: use "main" when branch detection fails
		integrationBranch = "main"
		a.Logger.Warn("could not detect current branch, using %q", integrationBranch)
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

	// Main loop
	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown
			stopNow := time.Now().UTC()
			_ = a.Emitter.EmitRunStopped(domain.RunStoppedData{Reason: "signal"}, stopNow)
			if !opts.Quiet {
				a.Logger.Info("amadeus run: stopped (signal)")
			}
			return nil
		default:
		}

		// Scan inbox
		consumed, scanErr := a.Store.ScanInbox(ctx)
		if scanErr != nil {
			a.Logger.Warn("inbox scan error: %v", scanErr)
			time.Sleep(opts.PollInterval)
			continue
		}

		if len(consumed) == 0 {
			time.Sleep(opts.PollInterval)
			continue
		}

		// Process consumed D-Mails
		inboxNow := time.Now().UTC()
		hasReport := false
		for _, d := range consumed {
			domain.LogBanner(a.Logger, domain.BannerRecv, string(d.Kind), d.Name, d.Description)
			if err := a.Emitter.EmitInboxConsumed(domain.InboxConsumedData{
				Name:   d.Name,
				Kind:   d.Kind,
				Source: d.Name + ".md",
			}, inboxNow); err != nil {
				a.Logger.Warn("emit inbox consumed: %v", err)
			}
			if d.Kind == domain.KindReport {
				hasReport = true
			}
		}

		if !opts.Quiet {
			a.Logger.Info("consumed %d D-Mail(s) from inbox", len(consumed))
		}

		// Trigger pre-merge pipeline on report D-Mails
		if hasReport {
			dmails, prErr := a.runPreMergePipeline(ctx, integrationBranch)
			if prErr != nil {
				a.Logger.Warn("pre-merge pipeline error: %v", prErr)
			} else if len(dmails) > 0 && !opts.Quiet {
				a.Logger.OK("generated %d implementation-feedback D-Mail(s)", len(dmails))
			}
		}

		// Trigger post-merge pipeline if BaseBranch is set and we got a report
		if hasReport && opts.BaseBranch != "" {
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
					// DriftError is expected, not a failure
					if _, ok := checkErr.(*domain.DriftError); !ok {
						a.Logger.Warn("post-merge check error: %v", checkErr)
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
