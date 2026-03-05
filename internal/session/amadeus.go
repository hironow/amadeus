package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/port"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Amadeus is the main orchestrator that wires Phase 1 (ReadingSteiner),
// Phase 2 (DivergenceMeter via Claude), and Phase 3 (D-Mail generation).
type Amadeus struct {
	Config      domain.Config
	Store       domain.StateReader
	Events      domain.EventStore   // nil skips event persistence (Projector still required for writes)
	Projector   domain.EventApplier // nil skips projection updates (Events still required for writes)
	Git         domain.Git
	RepoDir     string              // repository root directory
	Claude      port.ClaudeRunner // nil falls back to the default Claude runner
	Logger      domain.Logger
	DataOut     io.Writer              // machine-readable output (stdout); Logger is for human progress (stderr)
	Approver    port.Approver          // nil = no gate (auto-approve)
	Notifier    port.Notifier          // nil = no notifications
	Metrics     port.PolicyMetrics     // nil = no policy metrics
	ReviewCmd   string                 // code review command (empty = skip)
	ClaudeCmd   string                 // Claude CLI command (empty = "claude")
	ClaudeModel string                 // Claude model for review fix (empty = "opus")
	Aggregate   *domain.CheckAggregate // domain logic aggregate (injected by usecase layer)
	Dispatcher  port.EventDispatcher   // policy dispatch (injected by usecase layer; nil = no dispatch)
}

// claudeRunner returns the configured ClaudeRunner, falling back to the default Claude runner if nil.
func (a *Amadeus) claudeRunner() port.ClaudeRunner {
	if a.Claude != nil {
		return a.Claude
	}
	return DefaultClaudeRunner()
}

// emit appends events to the event store and applies them to projections.
// At least one of Events or Projector must be non-nil; otherwise emit returns
// an error to prevent silent data loss.
func (a *Amadeus) emit(events ...domain.Event) error {
	if a.Events == nil && a.Projector == nil {
		return fmt.Errorf("emit: neither EventStore nor Projector is configured — state would not be persisted")
	}
	if a.Events != nil {
		if err := a.Events.Append(events...); err != nil {
			return fmt.Errorf("append events: %w", err)
		}
	}
	if a.Projector != nil {
		for _, ev := range events {
			if err := a.Projector.Apply(ev); err != nil {
				return fmt.Errorf("apply event %s: %w", ev.Type, err)
			}
		}
	}
	// Dispatch events to policy handlers (best-effort: log and continue on error)
	if a.Dispatcher != nil {
		for _, ev := range events {
			if err := a.Dispatcher.Dispatch(context.Background(), ev); err != nil {
				if a.Logger != nil {
					a.Logger.Warn("policy dispatch %s: %v", ev.Type, err)
				}
			}
		}
	}
	return nil
}

// consumeInbox runs Phase 0: scans the inbox for inbound D-Mails and emits
// inbox-consumed events. Returns nil when there are no D-Mails to consume.
func (a *Amadeus) consumeInbox(ctx context.Context, quiet bool) error {
	span := trace.SpanFromContext(ctx)

	consumed, scanErr := a.Store.ScanInbox()
	if scanErr != nil {
		return fmt.Errorf("scan inbox: %w", scanErr)
	}
	if len(consumed) == 0 {
		return nil
	}
	if !quiet {
		a.Logger.Info("Consumed %d report(s) from inbox", len(consumed))
	}
	span.AddEvent("inbox.consumed", trace.WithAttributes(
		attribute.Int("inbox.count", len(consumed)),
	))
	now := time.Now().UTC()
	for _, d := range consumed {
		ev, evErr := domain.NewEvent(domain.EventInboxConsumed, domain.InboxConsumedData{
			Name:   d.Name,
			Kind:   d.Kind,
			Source: d.Name + ".md",
		}, now)
		if evErr != nil {
			return fmt.Errorf("create inbox event: %w", evErr)
		}
		if err := a.emit(ev); err != nil {
			return fmt.Errorf("emit inbox consumed: %w", err)
		}
	}
	return nil
}

// buildCheckPrompt runs Phase 2a: collects ADRs, DoDs, and dependency map,
// then builds the appropriate Claude prompt (full or diff).
func (a *Amadeus) buildCheckPrompt(report ShiftReport, fullCheck bool, previous domain.CheckResult, quiet bool) (string, error) {
	repoRoot := a.RepoDir
	allADRs, adrErr := CollectADRs(repoRoot)
	if adrErr != nil && !quiet {
		a.Logger.Info("Warning: failed to collect ADRs: %v", adrErr)
	}
	allDoDs, dodErr := CollectDoDs(repoRoot)
	if dodErr != nil && !quiet {
		a.Logger.Info("Warning: failed to collect DoDs: %v", dodErr)
	}
	depMap, depErr := CollectDependencyMap(repoRoot)
	if depErr != nil && !quiet {
		a.Logger.Info("Warning: failed to collect dependency map: %v", depErr)
	}

	if fullCheck {
		return platform.BuildFullCheckPrompt(a.Config.Lang, domain.FullCheckParams{
			CodebaseStructure: report.CodebaseStructure,
			AllADRs:           allADRs,
			RecentDoDs:        allDoDs,
			DependencyMap:     depMap,
		})
	}

	prevJSON, _ := json.Marshal(previous)
	var prTitles []string
	for _, pr := range report.MergedPRs {
		prTitles = append(prTitles, pr.Title)
	}
	issueIDs := domain.ExtractIssueIDs(prTitles...)
	linkedDoDs := ""
	if len(issueIDs) > 0 {
		linkedDoDs = allDoDs
	}
	return platform.BuildDiffCheckPrompt(a.Config.Lang, domain.DiffCheckParams{
		PreviousScores: string(prevJSON),
		PRDiffs:        report.Diff,
		RelevantADRs:   allADRs,
		LinkedDoDs:     linkedDoDs,
		LinkedIssueIDs: strings.Join(issueIDs, ", "),
	})
}

// runDivergenceMeter runs Phase 2b: executes Claude, parses the response,
// scores with DivergenceMeter, and handles divergence jump detection.
func (a *Amadeus) runDivergenceMeter(ctx context.Context, prompt string, fullCheck bool, previous domain.CheckResult, quiet bool) (domain.MeterResult, error) {
	_, span2 := platform.Tracer.Start(ctx, "divergence_meter") // nosemgrep: adr0003-otel-span-without-defer-end -- End() called per branch [permanent]

	// claude.invoke span wraps the Claude CLI execution with GenAI semconv attributes.
	model := a.ClaudeModel
	if model == "" {
		model = "opus"
	}
	invokeCtx, invokeSpan := platform.Tracer.Start(ctx, "claude.invoke",
		trace.WithAttributes(
			append([]attribute.KeyValue{
				attribute.String("claude.model", model),
			}, platform.GenAISpanAttrs(model)...)...,
		),
	)
	rawResp, err := a.claudeRunner().Run(invokeCtx, prompt)
	invokeSpan.End()
	if err != nil {
		span2.End()
		return domain.MeterResult{}, fmt.Errorf("phase 2 (claude): %w", err)
	}

	claudeResp, err := domain.ParseClaudeResponse(rawResp)
	if err != nil {
		span2.End()
		return domain.MeterResult{}, fmt.Errorf("phase 2 (parse): %w", err)
	}

	meter := &domain.DivergenceMeter{Config: a.Config}
	meterResult := meter.ProcessResponse(claudeResp)

	span2.AddEvent("divergence.evaluated", trace.WithAttributes(
		attribute.Float64("divergence.value", meterResult.Divergence.Value),
		attribute.String("divergence.severity", string(meterResult.Divergence.Severity)),
	))

	// Defer full scan to next run on large divergence jump
	if !fullCheck && a.Aggregate.ShouldPromoteToFull(previous.Divergence, meterResult.Divergence.Value) {
		span2.AddEvent("divergence.jump", trace.WithAttributes(
			attribute.Float64("divergence.previous", previous.Divergence),
			attribute.Float64("divergence.current", meterResult.Divergence.Value),
		))
		if !quiet {
			a.Logger.Info("Divergence jump detected (%.2f -> %.2f), next run will trigger full calibration",
				previous.Divergence, meterResult.Divergence.Value)
		}
		a.Aggregate.SetForceFullNext(true)
		ev, evErr := domain.NewEvent(domain.EventForceFullNextSet, domain.ForceFullNextSetData{
			PreviousDivergence: previous.Divergence,
			CurrentDivergence:  meterResult.Divergence.Value,
		}, time.Now().UTC())
		if evErr != nil {
			span2.End()
			return domain.MeterResult{}, fmt.Errorf("create force_full_next event: %w", evErr)
		}
		if err := a.emit(ev); err != nil {
			span2.End()
			return domain.MeterResult{}, fmt.Errorf("emit force_full_next: %w", err)
		}
	}
	span2.End()

	return meterResult, nil
}

// generateDMails runs Phase 3: creates D-Mail entities from meter candidates,
// validates them, and emits dmail-generated events.
func (a *Amadeus) generateDMails(ctx context.Context, meterResult domain.MeterResult, now time.Time) ([]domain.DMail, error) {
	_, span3 := platform.Tracer.Start(ctx, "dmail") // nosemgrep: adr0003-otel-span-without-defer-end -- End() called per branch [permanent]
	var dmails []domain.DMail
	for _, candidate := range meterResult.DMailCandidates {
		name, err := a.Store.NextDMailName(domain.KindFeedback)
		if err != nil {
			span3.End()
			return nil, fmt.Errorf("phase 3 (dmail name): %w", err)
		}
		dmail := domain.DMail{
			SchemaVersion: domain.DMailSchemaVersion,
			Name:          name,
			Kind:          domain.KindFeedback,
			Description:   candidate.Description,
			Issues:        candidate.Issues,
			Severity:      meterResult.Divergence.Severity,
			Action:        domain.DMailAction(candidate.Action),
			Targets:       candidate.Targets,
			Metadata: map[string]string{
				"created_at": now.Format(time.RFC3339),
			},
			Body: candidate.Detail,
		}
		if dmail.Action == "" {
			dmail.Action = domain.DefaultDMailAction(meterResult.Divergence.Severity)
		}
		if errs := domain.ValidateDMail(dmail); len(errs) > 0 {
			a.Logger.Warn("skipping invalid feedback dmail %s: %v", name, errs)
			continue
		}
		ev, evErr := domain.NewEvent(domain.EventDMailGenerated, domain.DMailGeneratedData{DMail: dmail}, now)
		if evErr != nil {
			span3.End()
			return nil, fmt.Errorf("phase 3 (create event): %w", evErr)
		}
		if err := a.emit(ev); err != nil {
			span3.End()
			return nil, fmt.Errorf("phase 3 (emit dmail): %w", err)
		}
		span3.AddEvent("dmail.created", trace.WithAttributes(
			attribute.String("dmail.name", dmail.Name),
			attribute.String("dmail.severity", string(dmail.Severity)),
		))
		dmails = append(dmails, dmail)
	}
	span3.End()
	return dmails, nil
}

// detectConvergence runs Phase 4: World Line Convergence detection.
// Analyzes all D-Mails for recurring patterns, emits convergence events,
// and generates convergence D-Mails for uncovered alerts.
func (a *Amadeus) detectConvergence(now time.Time) ([]domain.ConvergenceAlert, []domain.DMail, error) {
	allDMails, convergenceErr := a.Store.LoadAllDMails()
	if convergenceErr != nil {
		return nil, nil, nil // tolerate load failure
	}
	convergenceAlerts := domain.AnalyzeConvergence(allDMails, a.Config.Convergence, now)
	for _, alert := range convergenceAlerts {
		cev, cerr := domain.NewEvent(domain.EventConvergenceDetected, domain.ConvergenceDetectedData{
			Alert: alert,
		}, now)
		if cerr != nil {
			return nil, nil, fmt.Errorf("phase 4 (create convergence event): %w", cerr)
		}
		if err := a.emit(cev); err != nil {
			return nil, nil, fmt.Errorf("phase 4 (emit convergence event): %w", err)
		}
	}
	saved, saveErr := a.saveConvergenceDMails(convergenceAlerts)
	if saveErr != nil {
		return convergenceAlerts, nil, saveErr
	}
	return convergenceAlerts, saved, nil
}

// detectShift runs Phase 1: ReadingSteiner shift detection.
// Returns the shift report, whether a full check was performed, and any error.
func (a *Amadeus) detectShift(ctx context.Context, previous domain.CheckResult, fullMode bool, quiet bool) (ShiftReport, bool, error) {
	fullCheck := a.Aggregate.ShouldFullCheck(fullMode)
	if a.Aggregate.ForceFullNext() {
		if !quiet {
			a.Logger.Info("Full scan triggered by previous divergence jump")
		}
		a.Aggregate.SetForceFullNext(false) // consumed
	}

	rs := &ReadingSteiner{Git: a.Git}
	var report ShiftReport
	var err error

	_, span1 := platform.Tracer.Start(ctx, "reading_steiner") // nosemgrep: adr0003-otel-span-without-defer-end -- End() called per branch [permanent]
	if fullCheck {
		report, err = rs.DetectShiftFull(a.RepoDir)
		if err != nil {
			span1.End()
			return ShiftReport{}, fullCheck, fmt.Errorf("phase 1 (full): %w", err)
		}
	} else {
		sinceCommit := previous.Commit
		if sinceCommit == "" {
			fullCheck = true
			report, err = rs.DetectShiftFull(a.RepoDir)
			if err != nil {
				span1.End()
				return ShiftReport{}, fullCheck, fmt.Errorf("phase 1 (first run): %w", err)
			}
		} else {
			report, err = rs.DetectShift(sinceCommit)
			if err != nil {
				span1.End()
				return ShiftReport{}, fullCheck, fmt.Errorf("phase 1 (diff): %w", err)
			}
		}
	}
	parentSpan := trace.SpanFromContext(ctx)
	parentSpan.SetAttributes(attribute.Bool("check.full", fullCheck))
	if report.Significant {
		span1.AddEvent("shift.detected", trace.WithAttributes(
			attribute.Int("shift.pr_count", len(report.MergedPRs)),
		))
	}
	span1.End()

	return report, fullCheck, nil
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
	events, err := a.Events.LoadAll()
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
func (a *Amadeus) RunCheck(ctx context.Context, opts domain.CheckOptions) error {
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

	// Restore aggregate from persisted state (usecase layer sets up Aggregate)
	a.Aggregate.Restore(previous)

	// Phase 0: Consume inbox D-Mails (skip in dry-run to avoid mutating state)
	if !opts.DryRun {
		if err := a.consumeInbox(ctx, opts.Quiet); err != nil {
			return err
		}
	}

	report, fullCheck, err := a.detectShift(ctx, previous, opts.Full, opts.Quiet)
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
		a.Aggregate.AdvanceCheckCount(fullCheck)
		now := time.Now().UTC()
		noShiftResult := previous
		noShiftResult.Commit = currentCommit
		noShiftResult.CheckedAt = now
		noShiftResult.Type = domain.CheckTypeDiff
		noShiftResult.PRsEvaluated = nil
		noShiftResult.DMails = nil
		noShiftResult.ConvergenceAlerts = nil
		events, evErr := a.Aggregate.RecordCheck(noShiftResult, now)
		if evErr != nil {
			return fmt.Errorf("record check (no shift): %w", evErr)
		}
		if err := a.emit(events...); err != nil {
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

	prompt, err := a.buildCheckPrompt(report, fullCheck, previous, opts.Quiet)
	if err != nil {
		return fmt.Errorf("phase 2 (build prompt): %w", err)
	}

	if opts.DryRun {
		w := a.DataOut
		if w == nil {
			w = io.Discard
		}
		fmt.Fprintln(w, prompt)
		return nil
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
		a.Aggregate.AdvanceCheckCount(fullCheck)
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
		events, evErr := a.Aggregate.RecordCheck(gateDeniedResult, now)
		if evErr != nil {
			return fmt.Errorf("record check (gate denied): %w", evErr)
		}
		if err := a.emit(events...); err != nil {
			return fmt.Errorf("emit check (gate denied): %w", err)
		}
		platform.RecordCheck(ctx, "drift")
		if !opts.Quiet {
			a.Logger.Info("Gate denied — D-Mail generation skipped")
		}
		return nil
	}

	dmails, err := a.generateDMails(ctx, meterResult, now)
	if err != nil {
		return err
	}

	convergenceAlerts, convergenceDMails, err := a.detectConvergence(now)
	if err != nil {
		return err
	}
	dmails = append(dmails, convergenceDMails...)

	var prNumbers []string
	for _, pr := range report.MergedPRs {
		prNumbers = append(prNumbers, pr.Number)
	}
	var dmailNames []string
	for _, d := range dmails {
		dmailNames = append(dmailNames, d.Name)
	}

	a.Aggregate.AdvanceCheckCount(fullCheck)
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

	checkEvents, evErr := a.Aggregate.RecordCheck(result, now)
	if evErr != nil {
		return fmt.Errorf("record check: %w", evErr)
	}
	if err := a.emit(checkEvents...); err != nil {
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
		passed, revErr := RunReviewGate(ctx, a.ReviewCmd, a.ClaudeCmd, a.ClaudeModel, a.RepoDir, 300, a.Logger)
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

// saveConvergenceDMails creates and saves D-Mails for convergence alerts.
// Skips alerts whose target already has an existing convergence D-Mail in the archive
// to prevent duplicate messages on repeated runs.
// Returns the saved D-Mails and any error encountered during naming or writing.
func (a *Amadeus) saveConvergenceDMails(alerts []domain.ConvergenceAlert) ([]domain.DMail, error) {
	// Load existing D-Mails for dedup (I/O)
	allDMails, err := a.Store.LoadAllDMails()
	if err != nil {
		allDMails = nil // tolerate load failure; proceed without dedup
	}

	// Filter alerts using domain logic (pure)
	uncovered := domain.FilterUncoveredConvergenceAlerts(allDMails, alerts)

	// Generate D-Mails from uncovered alerts (pure)
	convergenceDMails := domain.GenerateConvergenceDMails(uncovered)

	// Persist generated D-Mails (I/O)
	var saved []domain.DMail
	for _, cd := range convergenceDMails {
		cdName, nameErr := a.Store.NextDMailName(domain.KindConvergence)
		if nameErr != nil {
			return saved, fmt.Errorf("convergence dmail name: %w", nameErr)
		}
		cd.Name = cdName
		ev, evErr := domain.NewEvent(domain.EventDMailGenerated, domain.DMailGeneratedData{DMail: cd}, time.Now().UTC())
		if evErr != nil {
			return saved, fmt.Errorf("create convergence event: %w", evErr)
		}
		if err := a.emit(ev); err != nil {
			return saved, fmt.Errorf("emit convergence dmail %s: %w", cdName, err)
		}
		saved = append(saved, cd)
	}
	return saved, nil
}

// MarkCommented records that a D-Mail x Issue pair has been posted as a comment.
func (a *Amadeus) MarkCommented(dmailName, issueID string) error {
	now := time.Now().UTC()
	ev, err := domain.NewEvent(domain.EventDMailCommented, domain.DMailCommentedData{
		DMail: dmailName, IssueID: issueID,
	}, now)
	if err != nil {
		return fmt.Errorf("create comment event: %w", err)
	}
	return a.emit(ev)
}

