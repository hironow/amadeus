package session

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/usecase/port"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Amadeus is the main orchestrator that wires Phase 1 (ReadingSteiner),
// Phase 2 (DivergenceMeter via Claude), and Phase 3 (D-Mail generation).
type Amadeus struct {
	Config      domain.Config
	Store       port.StateReader
	Events      port.EventStore     // nil skips event persistence (Projector still required for writes)
	Projector   domain.EventApplier // nil skips projection updates (Events still required for writes)
	Git         port.Git
	RepoDir     string            // repository root directory
	Claude      port.ClaudeRunner // nil falls back to the default Claude runner
	Logger      domain.Logger
	DataOut     io.Writer               // machine-readable output (stdout); Logger is for human progress (stderr)
	Approver    port.Approver           // nil = no gate (auto-approve)
	Notifier    port.Notifier           // nil = no notifications
	Metrics     port.PolicyMetrics      // nil = no policy metrics
	ReviewCmd   string                  // code review command (empty = skip)
	ClaudeCmd   string                  // Claude CLI command (set by cmd layer from config)
	ClaudeModel string                  // Claude model for review fix (set by cmd layer from config)
	PRReader    port.GitHubPRReader     // nil = skip PR convergence
	PRWriter    port.GitHubPRWriter     // nil = skip PR label writes
	PRPipeline  port.PRPipelineRunner   // nil = skip PR convergence (usecase-injected)
	IssueWriter port.GitHubIssueWriter  // nil = skip issue close
	Emitter     port.CheckEventEmitter  // event production + persistence + dispatch (injected by usecase layer)
	State       port.CheckStateProvider // aggregate state read/write (injected by usecase layer)
	SeqAlloc    port.SeqAllocator       // global SeqNr (ADR S0040)
	Insights    *InsightWriter          // nil = skip insight generation
	Collector   *ImprovementCollector   // nil = skip external improvement signal ingestion
	Policy      domain.RoutingPolicy    // corrective routing policy (loaded from YAML, fallback = default)

	// InboxCh overrides MonitorInbox when set (for testing).
	// When nil, Run starts MonitorInbox automatically.
	InboxCh <-chan domain.DMail
}

// claudeRunner returns the configured ClaudeRunner, falling back to the default Claude runner if nil.
// When using the default, wraps with SessionTrackingAdapter for session persistence.
func (a *Amadeus) claudeRunner() port.ClaudeRunner {
	if a.Claude != nil {
		return a.Claude
	}
	adapter := &ClaudeAdapter{ClaudeCmd: a.ClaudeCmd, Model: a.ClaudeModel, Logger: a.Logger}
	dbPath := filepath.Join(a.RepoDir, domain.StateDir, ".run", "sessions.db")
	store, err := NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		if a.Logger != nil {
			a.Logger.Debug("session tracking unavailable: %v", err)
		}
		return adapter
	}
	// Store lives for the duration of this Run; closed when amadeus exits.
	return NewSessionTrackingAdapter(adapter, store, domain.ProviderClaudeCode)
}

// runPreMergePipeline delegates to the PRPipeline port (usecase-injected).
// Returns nil when PRPipeline is nil (PR convergence disabled).
func (a *Amadeus) runPreMergePipeline(ctx context.Context, integrationBranch string) ([]domain.DMail, error) {
	if a.PRPipeline == nil {
		return nil, nil
	}
	return a.PRPipeline.RunPreMergePipeline(ctx, integrationBranch)
}

// SetPRPipeline injects the PR convergence pipeline runner.
func (a *Amadeus) SetPRPipeline(runner port.PRPipelineRunner) {
	a.PRPipeline = runner
}

// EventStore returns the event persistence store.
func (a *Amadeus) EventStore() port.EventStore {
	return a.Events
}

// EventApplier returns the projection applier.
func (a *Amadeus) EventApplier() domain.EventApplier {
	return a.Projector
}

func (a *Amadeus) SeqAllocator() port.SeqAllocator {
	return a.SeqAlloc
}

// autoRebuildIfNeeded checks if projections are missing but events exist,
// and rebuilds projections from the event store if so.
func (a *Amadeus) autoRebuildIfNeeded(quiet bool) error {
	if a.Events == nil || a.Projector == nil {
		return nil
	}
	latest, err := a.Store.LoadLatest()
	if err != nil {
		return fmt.Errorf("check latest for auto-rebuild: %w", err)
	}
	projectionEmpty := latest.CheckedAt.IsZero()
	if !projectionEmpty {
		return nil // projections exist, no rebuild needed
	}
	events, _, err := a.Events.LoadAll()
	if err != nil {
		return fmt.Errorf("load events for auto-rebuild: %w", err)
	}
	if len(events) == 0 {
		return nil // no events to replay
	}
	// Check for inbox-consumed events that would risk data loss on rebuild.
	hasInboxConsumed := false
	for _, ev := range events {
		if ev.Type == domain.EventInboxConsumed {
			hasInboxConsumed = true
			break
		}
	}
	if !domain.ShouldAutoRebuild(projectionEmpty, hasInboxConsumed) {
		if !quiet && hasInboxConsumed {
			a.Logger.Info("auto-rebuild skipped: inbox-consumed events exist; use 'amadeus rebuild' to avoid data loss")
		}
		return nil
	}
	if !quiet {
		a.Logger.Info("projections missing, rebuilding from %d event(s)", len(events))
	}
	if err := a.Projector.Rebuild(events); err != nil {
		return fmt.Errorf("auto-rebuild: %w", err)
	}
	return nil
}

// RunCheck executes the five-phase divergence check pipeline:
//   - Phase 0: Inbox consumption (scan inbound D-Mails)
//   - Phase 1: ReadingSteiner detects shifts (diff or full scan)
//   - Phase 2: Claude evaluates divergence, DivergenceMeter scores it
//   - Phase 3: D-Mail generation and routing
//   - Phase 4: World Line Convergence detection
//
// emitter and state are injected by the usecase layer (composition root wiring).
func (a *Amadeus) RunCheck(ctx context.Context, opts domain.CheckOptions, emitter port.CheckEventEmitter, state port.CheckStateProvider) error {
	if emitter != nil {
		a.Emitter = emitter
	}
	if state != nil {
		a.State = state
	}
	ctx, span := platform.Tracer.Start(ctx, "domain.check",
		trace.WithAttributes(
			attribute.Bool("check.dry_run", opts.DryRun),
		))
	defer span.End()

	// Auto-rebuild is a state-mutating operation (clears and rewrites projection
	// directories), so it must be skipped in dry-run mode.
	if !opts.DryRun {
		if err := a.autoRebuildIfNeeded(opts.Quiet); err != nil {
			return fmt.Errorf("auto-rebuild: %w", err)
		}
	}

	previous, err := a.Store.LoadLatest()
	if err != nil {
		return fmt.Errorf("load previous state: %w", err)
	}

	// Restore aggregate state from persisted projection
	a.State.Restore(previous)

	if a.Collector != nil {
		if _, err := a.Collector.PollOnce(ctx, 0); err != nil && a.Logger != nil {
			a.Logger.Warn("improvement collector: %v", err)
		}
	}

	// Phase 0: Consume inbox D-Mails (skip in dry-run to avoid mutating state).
	// Consumed D-Mails are passed to generateDMails for feedback_round propagation.
	var inboxDMails []domain.DMail
	if !opts.DryRun {
		// nosemgrep: adr0003-otel-span-without-defer-end -- End() called per branch [permanent]
		_, inboxSpan := platform.Tracer.Start(ctx, "phase.inbox_drain",
			trace.WithAttributes(
				attribute.Int("phase.number", 0),
				attribute.String("phase.name", "inbox_drain"),
			),
		)
		var consumeErr error
		inboxDMails, consumeErr = a.consumeInbox(ctx, opts.Quiet)
		if consumeErr != nil {
			inboxSpan.End()
			return consumeErr
		}
		inboxSpan.End()

		// Handle stall-escalation D-Mails from sightjack (SPEC-001).
		if stalls := ExtractStallEscalations(inboxDMails); len(stalls) > 0 {
			HandleStallEscalations(stalls, a.Logger)
		}
	}

	report, fullCheck, wasForced, err := a.detectShift(ctx, previous, opts.Full, opts.Quiet)
	if err != nil {
		return err
	}

	if !report.Significant {
		if !opts.Quiet {
			a.Logger.Info("Reading Steiner: no significant shift detected")
		}
		currentCommit, err := a.Git.CurrentCommit()
		if err != nil {
			return fmt.Errorf("get current commit: %w", err)
		}
		a.State.AdvanceCheckCount(fullCheck, wasForced)
		now := time.Now().UTC()
		noShiftResult := previous
		noShiftResult.Commit = currentCommit
		noShiftResult.CheckedAt = now
		noShiftResult.Type = domain.CheckTypeDiff
		noShiftResult.PRsEvaluated = nil
		noShiftResult.DMails = nil
		noShiftResult.ConvergenceAlerts = nil
		if err := a.Emitter.EmitCheck(noShiftResult, now); err != nil {
			return fmt.Errorf("emit check (no shift): %w", err)
		}
		platform.RecordCheck(ctx, "clean")
		if opts.JSON {
			if err := a.PrintCheckOutputJSON(previous, nil, previous.Divergence); err != nil {
				return fmt.Errorf("write JSON output: %w", err)
			}
		} else if opts.Quiet {
			a.dataOut("%s (%s) 0 D-Mails",
				domain.FormatDivergence(previous.Divergence*100),
				domain.FormatDelta(previous.Divergence, previous.Divergence))
		}
		return nil
	}

	if !opts.Quiet {
		a.Logger.Info("Reading Steiner: %d PRs merged since last check", len(report.MergedPRs))
		for _, pr := range report.MergedPRs {
			a.Logger.Info("  %s %s", pr.Number, pr.Title)
		}
	}

	prompt, cleanup, err := a.buildCheckPrompt(ctx, report, fullCheck, previous, opts.Quiet)
	if err != nil {
		return fmt.Errorf("phase 2 (build prompt): %w", err)
	}
	defer cleanup()

	if opts.DryRun {
		fmt.Fprintln(a.DataOut, prompt)
		return nil
	}

	if !opts.Quiet {
		a.Logger.Info("Divergence Meter: evaluating with %s...", a.ClaudeModel)
	}
	meterResult, err := a.runDivergenceMeter(ctx, prompt, fullCheck, previous, opts.Quiet)
	if err != nil {
		return err
	}

	currentCommit, err := a.Git.CurrentCommit()
	if err != nil {
		return fmt.Errorf("get current commit: %w", err)
	}
	now := time.Now().UTC()

	// Write divergence insight (best-effort, does not fail the check)
	commitRange := previous.Commit + ".." + currentCommit
	a.writeDivergenceInsight(meterResult.Divergence, currentCommit, commitRange, meterResult.Reasoning)

	// Gate: request approval before D-Mail generation
	gateApproved := true
	if len(meterResult.DMailCandidates) > 0 && a.Approver != nil {
		approved, gateErr := a.Approver.RequestApproval(ctx, fmt.Sprintf(
			"Drift detected (%.1f%%) — %d D-Mail(s) to generate. Approve?",
			meterResult.Divergence.Value*100, len(meterResult.DMailCandidates)))
		if gateErr != nil {
			return fmt.Errorf("approval gate: %w", gateErr)
		}
		gateApproved = approved
	}

	if !gateApproved {
		// Emit check.completed event to maintain ES invariant.
		a.State.AdvanceCheckCount(fullCheck, wasForced)
		checkType := domain.CheckTypeDiff
		if fullCheck {
			checkType = domain.CheckTypeFull
		}
		gateDeniedResult := domain.CheckResult{
			CheckedAt:  now,
			Commit:     currentCommit,
			Type:       checkType,
			Divergence: meterResult.Divergence.Value,
			Axes:       meterResult.Divergence.Axes,
			GateDenied: true,
		}
		if err := a.Emitter.EmitCheck(gateDeniedResult, now); err != nil {
			return fmt.Errorf("emit check (gate denied): %w", err)
		}
		platform.RecordCheck(ctx, "drift")
		if !opts.Quiet {
			a.Logger.Info("Gate denied — D-Mail generation skipped")
		}
		return nil
	}

	dmails, err := a.generateDMails(ctx, meterResult, inboxDMails, now)
	if err != nil {
		return err
	}
	a.writeImprovementOutcomeInsight(inboxDMails, currentCommit, len(dmails))

	// nosemgrep: adr0003-otel-span-without-defer-end -- End() called explicitly before error return [permanent]
	_, convSpan := platform.Tracer.Start(ctx, "phase.convergence_detection",
		trace.WithAttributes(
			attribute.Int("phase.number", 4),
			attribute.String("phase.name", "convergence_detection"),
		),
	)
	convergenceAlerts, convergenceDMails, err := a.detectConvergence(now)
	convSpan.End()
	if err != nil {
		return err
	}
	dmails = append(dmails, convergenceDMails...)

	// Write convergence insights for HIGH severity alerts (best-effort)
	for _, alert := range convergenceAlerts {
		a.writeConvergenceInsight(alert, currentCommit)
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
		ADRAlignment:      meterResult.Divergence.ADRAlignment, // E19: per-ADR scores
	}

	if err := a.Emitter.EmitCheck(result, now); err != nil {
		return fmt.Errorf("emit check completed: %w", err)
	}
	if len(dmails) > 0 {
		platform.RecordCheck(ctx, "drift")
	} else {
		platform.RecordCheck(ctx, "clean")
	}

	if opts.JSON {
		if err := a.PrintCheckOutputJSON(result, dmails, previous.Divergence); err != nil {
			return fmt.Errorf("write JSON output: %w", err)
		}
	} else if opts.Quiet {
		a.PrintCheckOutputQuiet(result, dmails, previous.Divergence)
	} else {
		a.PrintCheckOutput(result, dmails, previous.Divergence)
	}

	// Review Gate: run code review if configured
	if a.ReviewCmd != "" {
		passed, revErr := RunReviewGate(ctx, a.ReviewCmd, a.claudeRunner(), a.RepoDir, 300, a.Logger)
		if revErr != nil {
			a.Logger.Warn("Review gate error: %v", revErr)
		} else if !passed {
			a.Logger.Warn("Review gate: not resolved after %d cycles", maxReviewGateCycles)
		}
	}

	if len(dmails) > 0 {
		if a.Notifier != nil {
			_ = a.Notifier.Notify(ctx, "amadeus",
				fmt.Sprintf("Drift %.1f%% — %d D-Mail(s) sent",
					result.Divergence*100, len(dmails)))
		}
		return &domain.DriftError{Divergence: result.Divergence, DMails: len(dmails)}
	}
	return nil
}

// MarkCommented records that a D-Mail x Issue pair has been posted as a comment.
func (a *Amadeus) MarkCommented(dmailName, issueID string) error {
	return a.Emitter.EmitDMailCommented(dmailName, issueID, time.Now().UTC())
}
