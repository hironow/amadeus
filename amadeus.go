package amadeus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// DriftError is returned by RunCheck when drift is detected (D-Mails generated).
// Callers can use errors.As to distinguish drift from runtime errors.
type DriftError struct {
	Divergence float64
	DMails     int
}

func (e *DriftError) Error() string {
	return fmt.Sprintf("drift detected: divergence=%f, %d D-Mail(s)", e.Divergence, e.DMails)
}

// ExitCode maps an error to a process exit code.
//
//	nil        → 0 (success)
//	DriftError → 2 (drift detected)
//	other      → 1 (runtime error)
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var de *DriftError
	if errors.As(err, &de) {
		return 2
	}
	return 1
}

// Amadeus is the main orchestrator that wires Phase 1 (ReadingSteiner),
// Phase 2 (DivergenceMeter via Claude), and Phase 3 (D-Mail generation).
type Amadeus struct {
	Config        Config
	Store         *ProjectionStore
	Events        EventStore // nil skips event persistence (Projector still required for writes)
	Projector     *Projector // nil skips projection updates (Events still required for writes)
	Git           *GitClient
	Claude        ClaudeRunner // nil falls back to the default Claude runner
	Logger        *Logger
	DataOut       io.Writer // machine-readable output (stdout); Logger is for human progress (stderr)
	CheckCount    int       // number of diff checks since last full check
	ForceFullNext bool      // set when a divergence jump defers a full scan to the next run
}

// claudeRunner returns the configured ClaudeRunner, falling back to the default Claude runner if nil.
func (a *Amadeus) claudeRunner() ClaudeRunner {
	if a.Claude != nil {
		return a.Claude
	}
	return DefaultClaudeRunner()
}

// emit appends events to the event store and applies them to projections.
// At least one of Events or Projector must be non-nil; otherwise emit returns
// an error to prevent silent data loss.
func (a *Amadeus) emit(events ...Event) error {
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
	return nil
}

// autoRebuildIfNeeded checks if projections are missing but events exist,
// and rebuilds projections from the event store if so.
func (a *Amadeus) autoRebuildIfNeeded() error {
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
	a.Logger.Info("projections missing, rebuilding from %d event(s)", len(events))
	if err := a.Projector.Rebuild(events); err != nil {
		return fmt.Errorf("auto-rebuild: %w", err)
	}
	return nil
}

// CheckOptions controls how RunCheck operates.
type CheckOptions struct {
	Full   bool
	DryRun bool
	Quiet  bool
	JSON   bool
}

// ShouldFullCheck determines whether the next check should be a full scan.
// Returns true if forceFlag is set or the check count since last full check
// has reached the configured interval.
func (a *Amadeus) ShouldFullCheck(forceFlag bool) bool {
	if forceFlag || a.ForceFullNext {
		return true
	}
	return a.CheckCount >= a.Config.FullCheck.Interval
}

// RunCheck executes the five-phase divergence check pipeline:
//   - Phase 0: Inbox consumption (scan inbound D-Mails)
//   - Phase 1: ReadingSteiner detects shifts (diff or full scan)
//   - Phase 2: Claude evaluates divergence, DivergenceMeter scores it
//   - Phase 3: D-Mail generation and routing
//   - Phase 4: World Line Convergence detection
func (a *Amadeus) RunCheck(ctx context.Context, opts CheckOptions) error {
	ctx, span := tracer.Start(ctx, "amadeus.check",
		trace.WithAttributes(
			attribute.Bool("check.dry_run", opts.DryRun),
		))
	defer span.End()

	if err := a.autoRebuildIfNeeded(); err != nil {
		return fmt.Errorf("auto-rebuild: %w", err)
	}

	previous, err := a.Store.LoadLatest()
	if err != nil {
		return fmt.Errorf("load previous state: %w", err)
	}

	// Restore persisted state
	a.CheckCount = previous.CheckCountSinceFull
	a.ForceFullNext = previous.ForceFullNext

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
				ev, evErr := NewEvent(EventInboxConsumed, InboxConsumedData{
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

	fullCheck := a.ShouldFullCheck(opts.Full)
	if a.ForceFullNext {
		if !opts.Quiet {
			a.Logger.Info("Full scan triggered by previous divergence jump")
		}
		a.ForceFullNext = false // consumed
	}

	rs := &ReadingSteiner{Git: a.Git}
	var report ShiftReport

	_, span1 := tracer.Start(ctx, "reading_steiner")
	if fullCheck {
		report, err = rs.DetectShiftFull(a.Git.Dir)
		if err != nil {
			span1.End()
			return fmt.Errorf("phase 1 (full): %w", err)
		}
	} else {
		sinceCommit := previous.Commit
		if sinceCommit == "" {
			fullCheck = true
			report, err = rs.DetectShiftFull(a.Git.Dir)
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
		a.AdvanceCheckCount(fullCheck)
		if err := a.SaveCheckState(currentCommit, previous, time.Now().UTC()); err != nil {
			return fmt.Errorf("save check state: %w", err)
		}
		if opts.JSON {
			if err := a.PrintCheckOutputJSON(previous, nil, previous.Divergence); err != nil {
				return fmt.Errorf("write JSON output: %w", err)
			}
		} else if opts.Quiet {
			a.dataOut("%s (%s) 0 D-Mails",
				FormatDivergence(previous.Divergence*100),
				FormatDelta(previous.Divergence, previous.Divergence))
		}
		return nil
	}

	if !opts.Quiet {
		a.Logger.Info("Reading Steiner: %d PRs merged since last check", len(report.MergedPRs))
		for _, pr := range report.MergedPRs {
			a.Logger.Info("  %s %s", pr.Number, pr.Title)
		}
	}

	_, span2 := tracer.Start(ctx, "divergence_meter")

	repoRoot := a.Git.Dir
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
		prompt, err = BuildFullCheckPrompt(a.Config.Lang, FullCheckParams{
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
		issueIDs := ExtractIssueIDs(prTitles...)
		linkedDoDs := ""
		if len(issueIDs) > 0 {
			linkedDoDs = allDoDs
		}
		prompt, err = BuildDiffCheckPrompt(a.Config.Lang, DiffCheckParams{
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

	claudeResp, err := ParseClaudeResponse(rawResp)
	if err != nil {
		span2.End()
		return fmt.Errorf("phase 2 (parse): %w", err)
	}

	meter := &DivergenceMeter{Config: a.Config}
	meterResult := meter.ProcessResponse(claudeResp)

	span2.AddEvent("divergence.evaluated", trace.WithAttributes(
		attribute.Float64("divergence.value", meterResult.Divergence.Value),
		attribute.String("divergence.severity", string(meterResult.Divergence.Severity)),
	))

	// Defer full scan to next run on large divergence jump
	if !fullCheck && a.ShouldPromoteToFull(previous.Divergence, meterResult.Divergence.Value) {
		span2.AddEvent("divergence.jump", trace.WithAttributes(
			attribute.Float64("divergence.previous", previous.Divergence),
			attribute.Float64("divergence.current", meterResult.Divergence.Value),
		))
		if !opts.Quiet {
			a.Logger.Info("Divergence jump detected (%.2f → %.2f), next run will trigger full calibration",
				previous.Divergence, meterResult.Divergence.Value)
		}
		if err := a.FlagForceFullNext(previous.Divergence, meterResult.Divergence.Value); err != nil {
			return fmt.Errorf("flag force full next: %w", err)
		}
	}
	span2.End()

	currentCommit, err := a.Git.CurrentCommit()
	if err != nil {
		return fmt.Errorf("get current commit: %w", err)
	}
	now := time.Now().UTC()

	_, span3 := tracer.Start(ctx, "dmail")
	var dmails []DMail
	for _, candidate := range meterResult.DMailCandidates {
		name, err := a.Store.NextDMailName(KindFeedback)
		if err != nil {
			span3.End()
			return fmt.Errorf("phase 3 (dmail name): %w", err)
		}
		dmail := DMail{
			Name:        name,
			Kind:        KindFeedback,
			Description: candidate.Description,
			Issues:      candidate.Issues,
			Severity:    meterResult.Divergence.Severity,
			Targets:     candidate.Targets,
			Metadata: map[string]string{
				"created_at": now.Format(time.RFC3339),
			},
			Body: candidate.Detail,
		}
		ev, evErr := NewEvent(EventDMailGenerated, DMailGeneratedData{DMail: dmail}, now)
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
	var convergenceAlerts []ConvergenceAlert
	if convergenceErr == nil {
		convergenceAlerts = AnalyzeConvergence(allDMails, a.Config.Convergence, now)
		for _, alert := range convergenceAlerts {
			cev, cerr := NewEvent(EventConvergenceDetected, ConvergenceDetectedData{
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

	a.AdvanceCheckCount(fullCheck)
	checkType := CheckTypeDiff
	if fullCheck {
		checkType = CheckTypeFull
	}

	result := CheckResult{
		CheckedAt:           now,
		Commit:              currentCommit,
		Type:                checkType,
		Divergence:          meterResult.Divergence.Value,
		Axes:                meterResult.Divergence.Axes,
		ImpactRadius:        meterResult.ImpactRadius,
		PRsEvaluated:        prNumbers,
		DMails:              dmailNames,
		ConvergenceAlerts:   convergenceAlerts,
		CheckCountSinceFull: a.CheckCount,
		ForceFullNext:       a.ForceFullNext,
	}

	checkEv, evErr := NewEvent(EventCheckCompleted, CheckCompletedData{Result: result}, now)
	if evErr != nil {
		return fmt.Errorf("create check event: %w", evErr)
	}
	if err := a.emit(checkEv); err != nil {
		return fmt.Errorf("emit check completed: %w", err)
	}
	if fullCheck {
		baselineEv, bErr := NewEvent(EventBaselineUpdated, BaselineUpdatedData{
			Commit: currentCommit, Divergence: result.Divergence,
		}, now)
		if bErr != nil {
			return fmt.Errorf("create baseline event: %w", bErr)
		}
		if err := a.emit(baselineEv); err != nil {
			return fmt.Errorf("emit baseline updated: %w", err)
		}
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

	if len(dmails) > 0 {
		return &DriftError{Divergence: result.Divergence, DMails: len(dmails)}
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
func (a *Amadeus) PrintCheckOutput(result CheckResult, dmails []DMail, previousDivergence float64) {
	a.dataOut("")
	a.dataOut("Divergence: %s (%s)",
		FormatDivergence(result.Divergence*100),
		FormatDelta(result.Divergence, previousDivergence))

	axisOrder := []Axis{AxisADR, AxisDoD, AxisDependency, AxisImplicit}
	axisNames := map[Axis]string{
		AxisADR:        "ADR Integrity",
		AxisDoD:        "DoD Fulfillment",
		AxisDependency: "Dependency Integrity",
		AxisImplicit:   "Implicit Constraints",
	}

	for _, axis := range axisOrder {
		if score, ok := result.Axes[axis]; ok {
			weight := weightForAxis(axis, a.Config.Weights)
			contribution := float64(score.Score) * weight
			a.dataOut("  %-22s %s — %s",
				axisNames[axis]+":",
				FormatDivergence(contribution),
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
			case SeverityHigh:
				prefix = "[HIGH]"
			case SeverityMedium:
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
func (a *Amadeus) PrintCheckOutputJSON(result CheckResult, dmails []DMail, previousDivergence float64) error {
	convergenceAlerts := result.ConvergenceAlerts
	if convergenceAlerts == nil {
		convergenceAlerts = []ConvergenceAlert{}
	}
	output := struct {
		Divergence        float64            `json:"divergence"`
		Delta             float64            `json:"delta"`
		Axes              map[Axis]AxisScore `json:"axes"`
		ImpactRadius      []ImpactEntry      `json:"impact_radius"`
		DMails            []DMail            `json:"dmails"`
		ConvergenceAlerts []ConvergenceAlert `json:"convergence_alerts"`
	}{
		Divergence:        result.Divergence,
		Delta:             result.Divergence - previousDivergence,
		Axes:              result.Axes,
		ImpactRadius:      result.ImpactRadius,
		DMails:            dmails,
		ConvergenceAlerts: convergenceAlerts,
	}
	if output.DMails == nil {
		output.DMails = []DMail{}
	}
	if output.ImpactRadius == nil {
		output.ImpactRadius = []ImpactEntry{}
	}
	return a.writeDataJSON(output)
}

// PrintCheckOutputQuiet renders a single-line summary for --quiet mode.
func (a *Amadeus) PrintCheckOutputQuiet(result CheckResult, dmails []DMail, previousDivergence float64) {
	dmailLabel := "D-Mails"
	if len(dmails) == 1 {
		dmailLabel = "D-Mail"
	}

	convergenceStr := ""
	if len(result.ConvergenceAlerts) > 0 {
		convergenceStr = fmt.Sprintf(" %d convergence", len(result.ConvergenceAlerts))
	}

	a.dataOut("%s (%s) %d %s%s",
		FormatDivergence(result.Divergence*100),
		FormatDelta(result.Divergence, previousDivergence),
		len(dmails),
		dmailLabel,
		convergenceStr)
}

// ShouldPromoteToFull returns true when the absolute divergence change between
// the previous and current values exceeds the configured on_divergence_jump threshold.
// Both increases and decreases trigger recalibration.
func (a *Amadeus) ShouldPromoteToFull(previousDivergence, currentDivergence float64) bool {
	delta := currentDivergence - previousDivergence
	if delta < 0 {
		delta = -delta
	}
	return delta >= a.Config.FullCheck.OnDivergenceJump
}

// AdvanceCheckCount updates the internal check counter.
// If fullCheck is true, the counter resets to 0; otherwise it increments by 1.
func (a *Amadeus) AdvanceCheckCount(fullCheck bool) {
	if fullCheck {
		a.CheckCount = 0
	} else {
		a.CheckCount++
	}
}

// FlagForceFullNext marks that the next check should be a full scan.
// Called when a divergence jump is detected, deferring recalibration to the next run.
func (a *Amadeus) FlagForceFullNext(previousDivergence, currentDivergence float64) error {
	a.ForceFullNext = true
	ev, err := NewEvent(EventForceFullNextSet, ForceFullNextSetData{
		PreviousDivergence: previousDivergence,
		CurrentDivergence:  currentDivergence,
	}, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("create force_full_next event: %w", err)
	}
	return a.emit(ev)
}

// SaveCheckState persists an updated CheckResult preserving prior divergence data.
// Updates CheckedAt and appends to history so every check run is auditable.
// Used on the early-return path when no significant shift is detected,
// and also to persist ForceFullNext when a divergence jump defers a full scan.
func (a *Amadeus) SaveCheckState(commit string, previous CheckResult, checkedAt time.Time) error {
	previous.Commit = commit
	previous.CheckedAt = checkedAt
	previous.Type = CheckTypeDiff
	previous.PRsEvaluated = nil
	previous.DMails = nil
	previous.ConvergenceAlerts = nil
	previous.CheckCountSinceFull = a.CheckCount
	previous.ForceFullNext = a.ForceFullNext
	ev, err := NewEvent(EventCheckCompleted, CheckCompletedData{Result: previous}, checkedAt)
	if err != nil {
		return fmt.Errorf("create check event: %w", err)
	}
	return a.emit(ev)
}

// loadCheckHistory returns CheckResults extracted from the event store.
func (a *Amadeus) loadCheckHistory() ([]CheckResult, error) {
	if a.Events == nil {
		return nil, nil
	}
	events, err := a.Events.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}
	var results []CheckResult
	for _, ev := range events {
		if ev.Type != EventCheckCompleted {
			continue
		}
		var data CheckCompletedData
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
		if h.Type == CheckTypeFull {
			delta = "(baseline)"
		} else if i+1 < len(history) {
			delta = "(" + FormatDelta(h.Divergence, history[i+1].Divergence) + ")"
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
			FormatDivergence(h.Divergence*100),
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
			case SeverityHigh:
				severityTag = "[HIGH]"
			case SeverityMedium:
				severityTag = "[MED] "
			default:
				severityTag = "[LOW] "
			}
			a.dataOut("  %s  %s %-10s %s",
				d.Name,
				severityTag,
				string(DMailSent),
				d.Description)
		}
	}

	// Convergence alerts from current archive
	convergenceAlerts := AnalyzeConvergence(dmails, a.Config.Convergence, time.Now().UTC())
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
	Kind        DMailKind         `json:"kind"`
	Description string            `json:"description"`
	Issues      []string          `json:"issues,omitempty"`
	Severity    Severity          `json:"severity,omitempty"`
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
		consumed = []ConsumedRecord{}
	}
	if history == nil {
		history = []CheckResult{}
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
			Status:      string(DMailSent),
		}
	}

	convergenceAlerts := AnalyzeConvergence(dmails, a.Config.Convergence, time.Now().UTC())
	if convergenceAlerts == nil {
		convergenceAlerts = []ConvergenceAlert{}
	}

	output := struct {
		History           []CheckResult      `json:"history"`
		DMails            []dmailJSONView    `json:"dmails"`
		Consumed          []ConsumedRecord   `json:"consumed"`
		ConvergenceAlerts []ConvergenceAlert `json:"convergence_alerts"`
	}{
		History:           history,
		DMails:            views,
		Consumed:          consumed,
		ConvergenceAlerts: convergenceAlerts,
	}
	return a.writeDataJSON(output)
}

// LinkDMail associates a D-Mail with a Linear issue ID.
// Returns an error if the D-Mail is already linked.
// Skips alerts whose target already has an existing convergence D-Mail in the archive
// to prevent duplicate messages on repeated runs.
// Returns the saved D-Mails and any error encountered during naming or writing.
func (a *Amadeus) saveConvergenceDMails(alerts []ConvergenceAlert) ([]DMail, error) {
	// Build set of targets already covered by existing convergence D-Mails
	coveredTargets := make(map[string]bool)
	allDMails, err := a.Store.LoadAllDMails()
	if err == nil {
		for _, d := range allDMails {
			if d.Kind == KindConvergence {
				for _, t := range d.Targets {
					coveredTargets[t] = true
				}
			}
		}
	}

	convergenceDMails := GenerateConvergenceDMails(alerts)
	var saved []DMail
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

		cdName, err := a.Store.NextDMailName(KindConvergence)
		if err != nil {
			return saved, fmt.Errorf("convergence dmail name: %w", err)
		}
		cd.Name = cdName
		ev, evErr := NewEvent(EventDMailGenerated, DMailGeneratedData{DMail: cd}, time.Now().UTC())
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

// MarkCommented records that a D-Mail × Issue pair has been posted as a comment.
func (a *Amadeus) MarkCommented(dmailName, issueID string) error {
	now := time.Now().UTC()
	ev, err := NewEvent(EventDMailCommented, DMailCommentedData{
		DMail: dmailName, IssueID: issueID,
	}, now)
	if err != nil {
		return fmt.Errorf("create comment event: %w", err)
	}
	return a.emit(ev)
}

// PrintSync builds and outputs the sync status as JSON to DataOut.
// Lists D-Mail × Issue pairs that have not yet been posted as comments.
func (a *Amadeus) PrintSync() error {
	syncState, err := a.Store.LoadSyncState()
	if err != nil {
		return fmt.Errorf("load sync state: %w", err)
	}
	allDMails, err := a.Store.LoadAllDMails()
	if err != nil {
		return fmt.Errorf("load all dmails: %w", err)
	}

	var pendingComments []PendingComment
	for _, d := range allDMails {
		if len(d.Issues) == 0 {
			continue
		}
		for _, issueID := range d.Issues {
			key := d.Name + ":" + issueID
			if _, commented := syncState.CommentedDMails[key]; commented {
				continue
			}
			pendingComments = append(pendingComments, PendingComment{
				DMail:       d.Name,
				IssueID:     issueID,
				Status:      string(DMailSent),
				Description: d.Description,
			})
		}
	}
	if pendingComments == nil {
		pendingComments = []PendingComment{}
	}

	output := SyncOutput{
		PendingComments: pendingComments,
	}
	return a.writeDataJSON(output)
}

// weightForAxis returns the configured weight for a given axis.
func weightForAxis(axis Axis, w Weights) float64 {
	switch axis {
	case AxisADR:
		return w.ADRIntegrity
	case AxisDoD:
		return w.DoDFulfillment
	case AxisDependency:
		return w.DependencyIntegrity
	case AxisImplicit:
		return w.ImplicitConstraints
	default:
		return 0
	}
}
