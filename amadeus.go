package amadeus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Amadeus is the main orchestrator that wires Phase 1 (ReadingSteiner),
// Phase 2 (DivergenceMeter via Claude), and Phase 3 (D-Mail generation).
type Amadeus struct {
	Config        Config
	Store         *StateStore
	Git           *GitClient
	Logger        *Logger
	DataOut       io.Writer // machine-readable output (stdout); Logger is for human progress (stderr)
	CheckCount    int       // number of diff checks since last full check
	ForceFullNext bool      // set when a divergence jump defers a full scan to the next run
}

// CheckOptions controls how RunCheck operates.
type CheckOptions struct {
	Full   bool
	DryRun bool
	Quiet  bool
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

// RunCheck executes the three-phase divergence check pipeline:
//   - Phase 1: ReadingSteiner detects shifts (diff or full scan)
//   - Phase 2: Claude evaluates divergence, DivergenceMeter scores it
//   - Phase 3: D-Mail generation and routing
func (a *Amadeus) RunCheck(ctx context.Context, opts CheckOptions) error {
	ctx, span := tracer.Start(ctx, "amadeus.check",
		trace.WithAttributes(
			attribute.Bool("check.dry_run", opts.DryRun),
		))
	defer span.End()

	previous, err := a.Store.LoadLatest()
	if err != nil {
		return fmt.Errorf("load previous state: %w", err)
	}

	// Restore persisted state
	a.CheckCount = previous.CheckCountSinceFull
	a.ForceFullNext = previous.ForceFullNext

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
		if opts.Quiet {
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

	var prompt string
	if fullCheck {
		prompt, err = BuildFullCheckPrompt(FullCheckParams{
			CodebaseStructure: report.CodebaseStructure,
		})
	} else {
		prevJSON, _ := json.Marshal(previous)
		prompt, err = BuildDiffCheckPrompt(DiffCheckParams{
			PreviousScores: string(prevJSON),
			PRDiffs:        report.Diff,
		})
	}
	if err != nil {
		span2.End()
		return fmt.Errorf("phase 2 (build prompt): %w", err)
	}

	if opts.DryRun {
		fmt.Fprintln(a.DataOut, prompt)
		span2.End()
		return nil
	}

	rawResp, err := runClaude(ctx, prompt)
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
		a.FlagForceFullNext()
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
		id, err := a.Store.NextDMailID()
		if err != nil {
			span3.End()
			return fmt.Errorf("phase 3 (dmail id): %w", err)
		}
		dmail := DMail{
			ID:        id,
			Severity:  meterResult.Divergence.Severity,
			Target:    candidate.Target,
			Type:      candidate.Type,
			Summary:   candidate.Summary,
			Detail:    candidate.Detail,
			CreatedAt: now,
		}
		dmail = RouteDMail(dmail)
		if err := a.Store.SaveDMail(dmail); err != nil {
			span3.End()
			return fmt.Errorf("phase 3 (save dmail): %w", err)
		}
		span3.AddEvent("dmail.created", trace.WithAttributes(
			attribute.String("dmail.id", dmail.ID),
			attribute.String("dmail.severity", string(dmail.Severity)),
			attribute.String("dmail.target", string(dmail.Target)),
		))
		dmails = append(dmails, dmail)
	}
	span3.End()

	var prNumbers []string
	for _, pr := range report.MergedPRs {
		prNumbers = append(prNumbers, pr.Number)
	}
	var dmailIDs []string
	for _, d := range dmails {
		dmailIDs = append(dmailIDs, d.ID)
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
		PRsEvaluated:        prNumbers,
		DMails:              dmailIDs,
		CheckCountSinceFull: a.CheckCount,
		ForceFullNext:       a.ForceFullNext,
	}

	if err := a.Store.SaveLatest(result); err != nil {
		return fmt.Errorf("save latest: %w", err)
	}
	if fullCheck {
		if err := a.Store.SaveBaseline(result); err != nil {
			return fmt.Errorf("save baseline: %w", err)
		}
	}
	if err := a.Store.SaveHistory(result); err != nil {
		return fmt.Errorf("save history: %w", err)
	}

	if opts.Quiet {
		a.PrintCheckOutputQuiet(result, dmails, previous.Divergence)
	} else {
		a.PrintCheckOutput(result, dmails, previous.Divergence)
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

	if len(dmails) > 0 {
		a.dataOut("")
		a.dataOut("D-Mails:")
		pending := 0
		for _, d := range dmails {
			var prefix string
			switch d.Severity {
			case SeverityHigh:
				prefix = "[HIGH]"
				pending++
			case SeverityMedium:
				prefix = "[MED] "
			default:
				prefix = "[LOW] "
			}
			status := "sent"
			if d.Status == DMailPending {
				status = "awaiting approval"
			}
			a.dataOut("  %s %s %s → %s to %s",
				prefix, d.ID, d.Summary, status, string(d.Target))
		}
		if pending > 0 {
			a.dataOut("")
			a.dataOut("%d pending. Run `amadeus resolve <id> --approve` or `--reject`", pending)
		}
	}
}

// PrintCheckOutputQuiet renders a single-line summary for --quiet mode.
func (a *Amadeus) PrintCheckOutputQuiet(result CheckResult, dmails []DMail, previousDivergence float64) {
	pending := 0
	for _, d := range dmails {
		if d.Status == DMailPending {
			pending++
		}
	}
	dmailLabel := "D-Mails"
	if len(dmails) == 1 {
		dmailLabel = "D-Mail"
	}

	pendingStr := ""
	if pending > 0 {
		pendingStr = fmt.Sprintf(" (%d pending)", pending)
	}

	a.dataOut("%s (%s) %d %s%s",
		FormatDivergence(result.Divergence*100),
		FormatDelta(result.Divergence, previousDivergence),
		len(dmails),
		dmailLabel,
		pendingStr)
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
func (a *Amadeus) FlagForceFullNext() {
	a.ForceFullNext = true
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
	previous.CheckCountSinceFull = a.CheckCount
	previous.ForceFullNext = a.ForceFullNext
	if err := a.Store.SaveLatest(previous); err != nil {
		return err
	}
	return a.Store.SaveHistory(previous)
}

// ResolveDMail updates a pending D-Mail to approved or rejected status.
// action must be "approve" or "reject". reason is required for reject.
func (a *Amadeus) ResolveDMail(ctx context.Context, id string, action string, reason string) error {
	_, span := tracer.Start(ctx, "amadeus.resolve",
		trace.WithAttributes(
			attribute.String("dmail.id", id),
			attribute.String("resolve.action", action),
		))
	defer span.End()

	dmail, err := a.Store.LoadDMail(id)
	if err != nil {
		return err
	}
	if dmail.Status != DMailPending {
		return fmt.Errorf("D-Mail %s is already %s", id, dmail.Status)
	}

	now := time.Now().UTC()
	dmail.ResolvedAt = &now
	dmail.ResolvedAction = &action

	switch action {
	case "approve":
		dmail.Status = DMailApproved
	case "reject":
		if reason == "" {
			return fmt.Errorf("reject reason is required")
		}
		dmail.Status = DMailRejected
		dmail.RejectReason = &reason
	default:
		return fmt.Errorf("unknown action: %s (use --approve or --reject)", action)
	}

	if err := a.Store.SaveDMail(dmail); err != nil {
		return fmt.Errorf("save resolved dmail: %w", err)
	}

	span.AddEvent("dmail.resolved", trace.WithAttributes(
		attribute.String("dmail.id", id),
		attribute.String("dmail.status", string(dmail.Status)),
	))

	a.dataOut("D-Mail %s %s.", id, action+"d")
	a.dataOut("%s → %sd at %s", dmail.Summary, action, now.Format(time.RFC3339))
	return nil
}

// PrintLog renders the history and D-Mail log to DataOut.
func (a *Amadeus) PrintLog() error {
	history, err := a.Store.LoadHistory()
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
			a.dataOut("  %s  %s %-10s %s → %s",
				d.ID,
				severityTag,
				string(d.Status),
				d.Summary,
				string(d.Target))
		}
	}

	return nil
}

// LinkDMail associates a D-Mail with a Linear issue ID.
// Returns an error if the D-Mail is already linked.
func (a *Amadeus) LinkDMail(dmailID string, linearIssueID string) error {
	dmail, err := a.Store.LoadDMail(dmailID)
	if err != nil {
		return err
	}
	if dmail.LinearIssueID != nil {
		return fmt.Errorf("D-Mail %s is already linked to %s", dmailID, *dmail.LinearIssueID)
	}
	dmail.LinearIssueID = &linearIssueID
	if err := a.Store.SaveDMail(dmail); err != nil {
		return fmt.Errorf("save linked dmail: %w", err)
	}
	a.dataOut("D-Mail %s linked to %s", dmailID, linearIssueID)
	return nil
}

// PrintSync outputs unsynced D-Mails as JSON to the logger's writer.
func (a *Amadeus) PrintSync() error {
	unsynced, err := a.Store.LoadUnsyncedDMails()
	if err != nil {
		return fmt.Errorf("load unsynced dmails: %w", err)
	}
	output := struct {
		Unsynced []DMail `json:"unsynced"`
	}{
		Unsynced: unsynced,
	}
	if output.Unsynced == nil {
		output.Unsynced = []DMail{}
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sync output: %w", err)
	}
	fmt.Fprintln(a.DataOut, string(data))
	return nil
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
