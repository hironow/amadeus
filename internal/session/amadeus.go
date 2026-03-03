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
	if !latest.CheckedAt.IsZero() {
		return nil // projections exist, no rebuild needed
	}
	events, err := a.Events.LoadAll()
	if err != nil {
		return fmt.Errorf("load events for auto-rebuild: %w", err)
	}
	if len(events) == 0 {
		return nil // no events to replay
	}
	// Inbox-consumed events contain only metadata, not the full D-Mail content.
	// Rebuild clears archive/ and outbox/, so inbox-sourced D-Mails would be
	// permanently lost. Skip auto-rebuild and recommend explicit rebuild.
	for _, ev := range events {
		if ev.Type == domain.EventInboxConsumed {
			if !quiet {
				a.Logger.Info("auto-rebuild skipped: inbox-consumed events exist; use 'amadeus rebuild' to avoid data loss")
			}
			return nil
		}
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
		consumed, scanErr := a.Store.ScanInbox()
		if scanErr != nil {
			return fmt.Errorf("scan inbox: %w", scanErr)
		}
		if len(consumed) > 0 {
			if !opts.Quiet {
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
		}
	}

	fullCheck := a.Aggregate.ShouldFullCheck(opts.Full)
	if a.Aggregate.ForceFullNext() {
		if !opts.Quiet {
			a.Logger.Info("Full scan triggered by previous divergence jump")
		}
		a.Aggregate.SetForceFullNext(false) // consumed
	}

	rs := &ReadingSteiner{Git: a.Git}
	var report ShiftReport

	_, span1 := platform.Tracer.Start(ctx, "reading_steiner") // nosemgrep: adr0003-otel-span-without-defer-end -- End() called per branch
	if fullCheck {
		report, err = rs.DetectShiftFull(a.RepoDir)
		if err != nil {
			span1.End()
			return fmt.Errorf("phase 1 (full): %w", err)
		}
	} else {
		sinceCommit := previous.Commit
		if sinceCommit == "" {
			fullCheck = true
			report, err = rs.DetectShiftFull(a.RepoDir)
			if err != nil {
				span1.End()
				return fmt.Errorf("phase 1 (first run): %w", err)
			}
		} else {
			report, err = rs.DetectShift(sinceCommit)
			if err != nil {
				span1.End()
				return fmt.Errorf("phase 1 (diff): %w", err)
			}
		}
	}
	span.SetAttributes(attribute.Bool("check.full", fullCheck))
	if report.Significant {
		span1.AddEvent("shift.detected", trace.WithAttributes(
			attribute.Int("shift.pr_count", len(report.MergedPRs)),
		))
	}
	span1.End()

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

	_, span2 := platform.Tracer.Start(ctx, "divergence_meter") // nosemgrep: adr0003-otel-span-without-defer-end -- End() called per branch

	repoRoot := a.RepoDir
	allADRs, adrErr := CollectADRs(repoRoot)
	if adrErr != nil && !opts.Quiet {
		a.Logger.Info("Warning: failed to collect ADRs: %v", adrErr)
	}
	allDoDs, dodErr := CollectDoDs(repoRoot)
	if dodErr != nil && !opts.Quiet {
		a.Logger.Info("Warning: failed to collect DoDs: %v", dodErr)
	}
	depMap, depErr := CollectDependencyMap(repoRoot)
	if depErr != nil && !opts.Quiet {
		a.Logger.Info("Warning: failed to collect dependency map: %v", depErr)
	}

	var prompt string
	if fullCheck {
		prompt, err = platform.BuildFullCheckPrompt(a.Config.Lang, domain.FullCheckParams{
			CodebaseStructure: report.CodebaseStructure,
			AllADRs:           allADRs,
			RecentDoDs:        allDoDs,
			DependencyMap:     depMap,
		})
	} else {
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
		prompt, err = platform.BuildDiffCheckPrompt(a.Config.Lang, domain.DiffCheckParams{
			PreviousScores: string(prevJSON),
			PRDiffs:        report.Diff,
			RelevantADRs:   allADRs,
			LinkedDoDs:     linkedDoDs,
			LinkedIssueIDs: strings.Join(issueIDs, ", "),
		})
	}
	if err != nil {
		span2.End()
		return fmt.Errorf("phase 2 (build prompt): %w", err)
	}

	if opts.DryRun {
		w := a.DataOut
		if w == nil {
			w = io.Discard
		}
		fmt.Fprintln(w, prompt)
		span2.End()
		return nil
	}

	rawResp, err := a.claudeRunner().Run(ctx, prompt)
	if err != nil {
		span2.End()
		return fmt.Errorf("phase 2 (claude): %w", err)
	}

	claudeResp, err := domain.ParseClaudeResponse(rawResp)
	if err != nil {
		span2.End()
		return fmt.Errorf("phase 2 (parse): %w", err)
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
		if !opts.Quiet {
			a.Logger.Info("Divergence jump detected (%.2f -> %.2f), next run will trigger full calibration",
				previous.Divergence, meterResult.Divergence.Value)
		}
		a.Aggregate.SetForceFullNext(true)
		ev, evErr := domain.NewEvent(domain.EventForceFullNextSet, domain.ForceFullNextSetData{
			PreviousDivergence: previous.Divergence,
			CurrentDivergence:  meterResult.Divergence.Value,
		}, time.Now().UTC())
		if evErr != nil {
			return fmt.Errorf("create force_full_next event: %w", evErr)
		}
		if err := a.emit(ev); err != nil {
			return fmt.Errorf("emit force_full_next: %w", err)
		}
	}
	span2.End()

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

	_, span3 := platform.Tracer.Start(ctx, "dmail") // nosemgrep: adr0003-otel-span-without-defer-end -- End() called per branch and at line 464
	var dmails []domain.DMail
	for _, candidate := range meterResult.DMailCandidates {
		name, err := a.Store.NextDMailName(domain.KindFeedback)
		if err != nil {
			span3.End()
			return fmt.Errorf("phase 3 (dmail name): %w", err)
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
			switch meterResult.Divergence.Severity {
			case domain.SeverityHigh:
				dmail.Action = domain.ActionEscalate
			case domain.SeverityMedium:
				dmail.Action = domain.ActionRetry
			default:
				dmail.Action = domain.ActionResolve
			}
		}
		if errs := domain.ValidateDMail(dmail); len(errs) > 0 {
			a.Logger.Warn("skipping invalid feedback dmail %s: %v", name, errs)
			continue
		}
		ev, evErr := domain.NewEvent(domain.EventDMailGenerated, domain.DMailGeneratedData{DMail: dmail}, now)
		if evErr != nil {
			span3.End()
			return fmt.Errorf("phase 3 (create event): %w", evErr)
		}
		if err := a.emit(ev); err != nil {
			span3.End()
			return fmt.Errorf("phase 3 (emit dmail): %w", err)
		}
		span3.AddEvent("dmail.created", trace.WithAttributes(
			attribute.String("dmail.name", dmail.Name),
			attribute.String("dmail.severity", string(dmail.Severity)),
		))
		dmails = append(dmails, dmail)
	}
	span3.End()

	// Phase 4: World Line Convergence detection
	allDMails, convergenceErr := a.Store.LoadAllDMails()
	var convergenceAlerts []domain.ConvergenceAlert
	if convergenceErr == nil {
		convergenceAlerts = domain.AnalyzeConvergence(allDMails, a.Config.Convergence, now)
		for _, alert := range convergenceAlerts {
			cev, cerr := domain.NewEvent(domain.EventConvergenceDetected, domain.ConvergenceDetectedData{
				Alert: alert,
			}, now)
			if cerr != nil {
				return fmt.Errorf("phase 4 (create convergence event): %w", cerr)
			}
			if err := a.emit(cev); err != nil {
				return fmt.Errorf("phase 4 (emit convergence event): %w", err)
			}
		}
		saved, saveErr := a.saveConvergenceDMails(convergenceAlerts)
		if saveErr != nil {
			return saveErr
		}
		dmails = append(dmails, saved...)
	}

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

// dataOut writes a formatted line to DataOut (stdout / machine-facing).
func (a *Amadeus) dataOut(format string, args ...any) {
	w := a.DataOut
	if w == nil {
		w = io.Discard
	}
	fmt.Fprintf(w, "  "+format+"\n", args...)
}

// writeDataJSON marshals v as indented JSON and writes it to DataOut.
func (a *Amadeus) writeDataJSON(v any) error {
	w := a.DataOut
	if w == nil {
		w = io.Discard
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Fprintln(w, string(data))
	return nil
}

// PrintCheckOutput renders the CLI display for a completed check.
func (a *Amadeus) PrintCheckOutput(result domain.CheckResult, dmails []domain.DMail, previousDivergence float64) {
	a.dataOut("")
	a.dataOut("Divergence: %s (%s)",
		domain.FormatDivergence(result.Divergence*100),
		domain.FormatDelta(result.Divergence, previousDivergence))

	axisOrder := []domain.Axis{domain.AxisADR, domain.AxisDoD, domain.AxisDependency, domain.AxisImplicit}
	axisNames := map[domain.Axis]string{
		domain.AxisADR:        "ADR Integrity",
		domain.AxisDoD:        "DoD Fulfillment",
		domain.AxisDependency: "Dependency Integrity",
		domain.AxisImplicit:   "Implicit Constraints",
	}

	for _, axis := range axisOrder {
		if score, ok := result.Axes[axis]; ok {
			weight := weightForAxis(axis, a.Config.Weights)
			contribution := float64(score.Score) * weight
			a.dataOut("  %-22s %s — %s",
				axisNames[axis]+":",
				domain.FormatDivergence(contribution),
				score.Details)
		}
	}

	if len(result.ImpactRadius) > 0 {
		a.dataOut("")
		a.dataOut("Impact Radius:")
		for _, entry := range result.ImpactRadius {
			a.dataOut("  [%s] %s — %s", entry.Impact, entry.Area, entry.Detail)
		}
	}

	if len(dmails) > 0 {
		a.dataOut("")
		a.dataOut("D-Mails:")
		for _, d := range dmails {
			var prefix string
			switch d.Severity {
			case domain.SeverityHigh:
				prefix = "[HIGH]"
			case domain.SeverityMedium:
				prefix = "[MED] "
			default:
				prefix = "[LOW] "
			}
			a.dataOut("  %s %s %s → sent",
				prefix, d.Name, d.Description)
		}
	}

	if len(result.ConvergenceAlerts) > 0 {
		a.dataOut("")
		a.dataOut("Convergence Alerts:")
		for _, alert := range result.ConvergenceAlerts {
			a.dataOut("  [%s] %s — %d hits in %d days (%d D-Mails)",
				strings.ToUpper(string(alert.Severity)),
				alert.Target,
				alert.Count,
				alert.Window,
				len(alert.DMails))
		}
	}
}

// PrintCheckOutputJSON writes the check result as JSON to DataOut.
func (a *Amadeus) PrintCheckOutputJSON(result domain.CheckResult, dmails []domain.DMail, previousDivergence float64) error {
	convergenceAlerts := result.ConvergenceAlerts
	if convergenceAlerts == nil {
		convergenceAlerts = []domain.ConvergenceAlert{}
	}
	output := struct {
		Divergence        float64                          `json:"divergence"`
		Delta             float64                          `json:"delta"`
		Axes              map[domain.Axis]domain.AxisScore `json:"axes"`
		ImpactRadius      []domain.ImpactEntry             `json:"impact_radius"`
		DMails            []domain.DMail                   `json:"dmails"`
		ConvergenceAlerts []domain.ConvergenceAlert        `json:"convergence_alerts"`
	}{
		Divergence:        result.Divergence,
		Delta:             result.Divergence - previousDivergence,
		Axes:              result.Axes,
		ImpactRadius:      result.ImpactRadius,
		DMails:            dmails,
		ConvergenceAlerts: convergenceAlerts,
	}
	if output.DMails == nil {
		output.DMails = []domain.DMail{}
	}
	if output.ImpactRadius == nil {
		output.ImpactRadius = []domain.ImpactEntry{}
	}
	return a.writeDataJSON(output)
}

// PrintCheckOutputQuiet renders a single-line summary for --quiet mode.
func (a *Amadeus) PrintCheckOutputQuiet(result domain.CheckResult, dmails []domain.DMail, previousDivergence float64) {
	dmailLabel := "D-Mails"
	if len(dmails) == 1 {
		dmailLabel = "D-Mail"
	}

	convergenceStr := ""
	if len(result.ConvergenceAlerts) > 0 {
		convergenceStr = fmt.Sprintf(" %d convergence", len(result.ConvergenceAlerts))
	}

	a.dataOut("%s (%s) %d %s%s",
		domain.FormatDivergence(result.Divergence*100),
		domain.FormatDelta(result.Divergence, previousDivergence),
		len(dmails),
		dmailLabel,
		convergenceStr)
}

// loadCheckHistory returns CheckResults extracted from the event store.
func (a *Amadeus) loadCheckHistory() ([]domain.CheckResult, error) {
	if a.Events == nil {
		return nil, nil
	}
	events, err := a.Events.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}
	var results []domain.CheckResult
	for _, ev := range events {
		if ev.Type != domain.EventCheckCompleted {
			continue
		}
		var data domain.CheckCompletedData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return nil, fmt.Errorf("unmarshal check event %s: %w", ev.ID, err)
		}
		results = append(results, data.Result)
	}
	// Events are chronological; history is newest-first
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}
	return results, nil
}

// PrintLog renders the history and D-Mail log to DataOut.
func (a *Amadeus) PrintLog() error {
	history, err := a.loadCheckHistory()
	if err != nil {
		return fmt.Errorf("load history: %w", err)
	}

	a.dataOut("")
	if len(history) == 0 {
		a.dataOut("No history yet. Run `amadeus check` first.")
		return nil
	}

	a.dataOut("History:")
	for i, h := range history {
		var delta string
		if h.Type == domain.CheckTypeFull {
			delta = "(baseline)"
		} else if i+1 < len(history) {
			delta = "(" + domain.FormatDelta(h.Divergence, history[i+1].Divergence) + ")"
		} else {
			delta = "(first)"
		}
		dmailCount := len(h.DMails)
		dmailLabel := "D-Mails"
		if dmailCount == 1 {
			dmailLabel = "D-Mail"
		}
		a.dataOut("  %s  %s  %-4s  %s %s  %d %s",
			h.CheckedAt.Format("2006-01-02T15:04"),
			h.Commit,
			string(h.Type),
			domain.FormatDivergence(h.Divergence*100),
			delta,
			dmailCount,
			dmailLabel)
	}

	dmails, err := a.Store.LoadAllDMails()
	if err != nil {
		return fmt.Errorf("load dmails: %w", err)
	}

	if len(dmails) > 0 {
		a.dataOut("")
		a.dataOut("D-Mails:")
		for _, d := range dmails {
			var severityTag string
			switch d.Severity {
			case domain.SeverityHigh:
				severityTag = "[HIGH]"
			case domain.SeverityMedium:
				severityTag = "[MED] "
			default:
				severityTag = "[LOW] "
			}
			a.dataOut("  %s  %s %-10s %s",
				d.Name,
				severityTag,
				string(domain.DMailSent),
				d.Description)
		}
	}

	// Convergence alerts from current archive
	convergenceAlerts := domain.AnalyzeConvergence(dmails, a.Config.Convergence, time.Now().UTC())
	if len(convergenceAlerts) > 0 {
		a.dataOut("")
		a.dataOut("Convergence Alerts:")
		for _, alert := range convergenceAlerts {
			a.dataOut("  [%s] %s — %d hits in %d days (%d D-Mails)",
				strings.ToUpper(string(alert.Severity)),
				alert.Target,
				alert.Count,
				alert.Window,
				len(alert.DMails))
		}
	}

	consumed, err := a.Store.LoadConsumed()
	if err != nil {
		return fmt.Errorf("load consumed: %w", err)
	}
	if len(consumed) > 0 {
		a.dataOut("")
		a.dataOut("Consumed:")
		for _, c := range consumed {
			a.dataOut("  %s  [%s]  %s",
				c.Name,
				string(c.Kind),
				c.ConsumedAt.Format("2006-01-02T15:04"))
		}
	}

	return nil
}

// dmailJSONView is a JSON-specific view of a D-Mail with status.
type dmailJSONView struct {
	Name        string            `json:"name"`
	Kind        domain.DMailKind  `json:"kind"`
	Description string            `json:"description"`
	Issues      []string          `json:"issues,omitempty"`
	Severity    domain.Severity   `json:"severity,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Status      string            `json:"status"`
}

// PrintLogJSON writes the history and D-Mail log as JSON to DataOut.
func (a *Amadeus) PrintLogJSON() error {
	history, err := a.loadCheckHistory()
	if err != nil {
		return fmt.Errorf("load history: %w", err)
	}
	dmails, err := a.Store.LoadAllDMails()
	if err != nil {
		return fmt.Errorf("load dmails: %w", err)
	}
	consumed, err := a.Store.LoadConsumed()
	if err != nil {
		return fmt.Errorf("load consumed: %w", err)
	}
	if consumed == nil {
		consumed = []domain.ConsumedRecord{}
	}
	if history == nil {
		history = []domain.CheckResult{}
	}

	views := make([]dmailJSONView, len(dmails))
	for i, d := range dmails {
		views[i] = dmailJSONView{
			Name:        d.Name,
			Kind:        d.Kind,
			Description: d.Description,
			Issues:      d.Issues,
			Severity:    d.Severity,
			Metadata:    d.Metadata,
			Status:      string(domain.DMailSent),
		}
	}

	convergenceAlerts := domain.AnalyzeConvergence(dmails, a.Config.Convergence, time.Now().UTC())
	if convergenceAlerts == nil {
		convergenceAlerts = []domain.ConvergenceAlert{}
	}

	output := struct {
		History           []domain.CheckResult      `json:"history"`
		DMails            []dmailJSONView           `json:"dmails"`
		Consumed          []domain.ConsumedRecord   `json:"consumed"`
		ConvergenceAlerts []domain.ConvergenceAlert `json:"convergence_alerts"`
	}{
		History:           history,
		DMails:            views,
		Consumed:          consumed,
		ConvergenceAlerts: convergenceAlerts,
	}
	return a.writeDataJSON(output)
}

// saveConvergenceDMails creates and saves D-Mails for convergence alerts.
// Skips alerts whose target already has an existing convergence D-Mail in the archive
// to prevent duplicate messages on repeated runs.
// Returns the saved D-Mails and any error encountered during naming or writing.
func (a *Amadeus) saveConvergenceDMails(alerts []domain.ConvergenceAlert) ([]domain.DMail, error) {
	// Build set of targets already covered by existing convergence D-Mails
	coveredTargets := make(map[string]bool)
	allDMails, err := a.Store.LoadAllDMails()
	if err == nil {
		for _, d := range allDMails {
			if d.Kind == domain.KindConvergence {
				for _, t := range d.Targets {
					coveredTargets[t] = true
				}
			}
		}
	}

	convergenceDMails := domain.GenerateConvergenceDMails(alerts)
	var saved []domain.DMail
	for _, cd := range convergenceDMails {
		// Skip if all targets are already covered
		allCovered := true
		for _, t := range cd.Targets {
			if !coveredTargets[t] {
				allCovered = false
				break
			}
		}
		if allCovered {
			continue
		}

		cdName, err := a.Store.NextDMailName(domain.KindConvergence)
		if err != nil {
			return saved, fmt.Errorf("convergence dmail name: %w", err)
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
		// Mark newly saved targets as covered
		for _, t := range cd.Targets {
			coveredTargets[t] = true
		}
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

// PrintSync builds and outputs the sync status as JSON to DataOut.
// Lists D-Mail x Issue pairs that have not yet been posted as comments.
func (a *Amadeus) PrintSync() error {
	syncState, err := a.Store.LoadSyncState()
	if err != nil {
		return fmt.Errorf("load sync state: %w", err)
	}
	allDMails, err := a.Store.LoadAllDMails()
	if err != nil {
		return fmt.Errorf("load all dmails: %w", err)
	}

	var pendingComments []domain.PendingComment
	for _, d := range allDMails {
		if len(d.Issues) == 0 {
			continue
		}
		for _, issueID := range d.Issues {
			key := d.Name + ":" + issueID
			if _, commented := syncState.CommentedDMails[key]; commented {
				continue
			}
			pendingComments = append(pendingComments, domain.PendingComment{
				DMail:       d.Name,
				IssueID:     issueID,
				Status:      string(domain.DMailSent),
				Description: d.Description,
			})
		}
	}
	if pendingComments == nil {
		pendingComments = []domain.PendingComment{}
	}

	output := domain.SyncOutput{
		PendingComments: pendingComments,
	}
	return a.writeDataJSON(output)
}

// weightForAxis returns the configured weight for a given axis.
func weightForAxis(axis domain.Axis, w domain.Weights) float64 {
	switch axis {
	case domain.AxisADR:
		return w.ADRIntegrity
	case domain.AxisDoD:
		return w.DoDFulfillment
	case domain.AxisDependency:
		return w.DependencyIntegrity
	case domain.AxisImplicit:
		return w.ImplicitConstraints
	default:
		return 0
	}
}
