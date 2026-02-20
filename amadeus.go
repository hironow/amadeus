package amadeus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	// Phase 0: Consume inbox D-Mails
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
		var records []ConsumedRecord
		for _, d := range consumed {
			records = append(records, ConsumedRecord{
				Name:       d.Name,
				Kind:       d.Kind,
				ConsumedAt: now,
				Source:     d.Name + ".md",
			})
		}
		if err := a.Store.SaveConsumed(records); err != nil {
			return fmt.Errorf("save consumed: %w", err)
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
		w := a.DataOut
		if w == nil {
			w = io.Discard
		}
		fmt.Fprintln(w, prompt)
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
			Metadata: map[string]string{
				"created_at": now.Format(time.RFC3339),
			},
			Body: candidate.Detail,
		}
		if err := a.Store.SaveDMail(dmail); err != nil {
			span3.End()
			return fmt.Errorf("phase 3 (save dmail): %w", err)
		}
		span3.AddEvent("dmail.created", trace.WithAttributes(
			attribute.String("dmail.name", dmail.Name),
			attribute.String("dmail.severity", string(dmail.Severity)),
		))
		dmails = append(dmails, dmail)
	}
	span3.End()

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
		PRsEvaluated:        prNumbers,
		DMails:              dmailNames,
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

	if len(dmails) > 0 {
		a.dataOut("")
		a.dataOut("D-Mails:")
		pending := 0
		for _, d := range dmails {
			var prefix string
			routeStatus := RouteDMail(d.Severity)
			switch d.Severity {
			case SeverityHigh:
				prefix = "[HIGH]"
				if routeStatus == DMailPending {
					pending++
				}
			case SeverityMedium:
				prefix = "[MED] "
			default:
				prefix = "[LOW] "
			}
			status := "sent"
			if routeStatus == DMailPending {
				status = "awaiting approval"
			}
			a.dataOut("  %s %s %s → %s",
				prefix, d.Name, d.Description, status)
		}
		if pending > 0 {
			a.dataOut("")
			a.dataOut("%d pending. Run `amadeus resolve <name> --approve` or `--reject`", pending)
		}
	}
}

// PrintCheckOutputJSON writes the check result as JSON to DataOut.
func (a *Amadeus) PrintCheckOutputJSON(result CheckResult, dmails []DMail, previousDivergence float64) error {
	output := struct {
		Divergence float64            `json:"divergence"`
		Delta      float64            `json:"delta"`
		Axes       map[Axis]AxisScore `json:"axes"`
		DMails     []DMail            `json:"dmails"`
	}{
		Divergence: result.Divergence,
		Delta:      result.Divergence - previousDivergence,
		Axes:       result.Axes,
		DMails:     dmails,
	}
	if output.DMails == nil {
		output.DMails = []DMail{}
	}
	return a.writeDataJSON(output)
}

// PrintCheckOutputQuiet renders a single-line summary for --quiet mode.
func (a *Amadeus) PrintCheckOutputQuiet(result CheckResult, dmails []DMail, previousDivergence float64) {
	pending := 0
	for _, d := range dmails {
		if RouteDMail(d.Severity) == DMailPending {
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

// resolveDMailCore performs the common resolution logic: load D-Mail from archive,
// validate eligibility, create Resolution sidecar entry, save.
// The D-Mail .md file itself is immutable; only the Resolution sidecar is updated.
func (a *Amadeus) resolveDMailCore(ctx context.Context, name, action, reason string) (DMail, Resolution, trace.Span, error) {
	_, span := tracer.Start(ctx, "amadeus.resolve",
		trace.WithAttributes(
			attribute.String("dmail.name", name),
			attribute.String("resolve.action", action),
		))

	dmail, err := a.Store.LoadDMail(name)
	if err != nil {
		return DMail{}, Resolution{}, span, err
	}

	// Check if already resolved.
	// Distinguish "not found" (ok to proceed) from read/parse errors (must surface).
	existing, err := a.Store.LoadResolution(name)
	if err == nil && existing.Status != "" {
		return DMail{}, Resolution{}, span, fmt.Errorf("D-Mail %s is already %s", name, existing.Status)
	}
	if err != nil && !errors.Is(err, ErrNoResolution) {
		return DMail{}, Resolution{}, span, fmt.Errorf("load resolution: %w", err)
	}

	// Only HIGH severity DMails are pending (eligible for resolution)
	if RouteDMail(dmail.Severity) != DMailPending {
		return DMail{}, Resolution{}, span, fmt.Errorf("D-Mail %s is not pending (severity: %s)", name, dmail.Severity)
	}

	now := time.Now().UTC()
	var resolution Resolution

	switch action {
	case "approve":
		resolution = Resolution{
			Name:       name,
			Status:     string(DMailApproved),
			Action:     action,
			ResolvedAt: &now,
		}
	case "reject":
		if reason == "" {
			return DMail{}, Resolution{}, span, fmt.Errorf("reject reason is required")
		}
		resolution = Resolution{
			Name:       name,
			Status:     string(DMailRejected),
			Action:     action,
			Reason:     reason,
			ResolvedAt: &now,
		}
	default:
		return DMail{}, Resolution{}, span, fmt.Errorf("unknown action: %s (use --approve or --reject)", action)
	}

	// Move file from pending/ before persisting resolution.
	// If move fails, no resolution is saved — avoids orphan resolution
	// that would block future resolve attempts.
	switch action {
	case "approve":
		if err := a.Store.MovePendingToOutbox(name); err != nil {
			return DMail{}, Resolution{}, span, fmt.Errorf("move to outbox: %w", err)
		}
	case "reject":
		if err := a.Store.MovePendingToRejected(name); err != nil {
			return DMail{}, Resolution{}, span, fmt.Errorf("move to rejected: %w", err)
		}
	}

	if err := a.Store.SaveResolution(resolution); err != nil {
		return DMail{}, Resolution{}, span, fmt.Errorf("save resolution: %w", err)
	}

	span.AddEvent("dmail.resolved", trace.WithAttributes(
		attribute.String("dmail.name", name),
		attribute.String("dmail.status", resolution.Status),
	))

	return dmail, resolution, span, nil
}

// ResolveDMail updates a pending D-Mail to approved or rejected status.
// action must be "approve" or "reject". reason is required for reject.
func (a *Amadeus) ResolveDMail(ctx context.Context, name string, action string, reason string) error {
	dmail, resolution, span, err := a.resolveDMailCore(ctx, name, action, reason)
	defer span.End()
	if err != nil {
		return err
	}
	a.dataOut("D-Mail %s %s.", name, action+"d")
	a.dataOut("%s → %sd at %s", dmail.Description, action, resolution.ResolvedAt.Format(time.RFC3339))
	return nil
}

// ResolveOutput is the JSON-serializable result of resolving a D-Mail.
type ResolveOutput struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Action     string `json:"action"`
	ResolvedAt string `json:"resolved_at"`
}

// ResolveDMailResult resolves a D-Mail and returns the result as a struct.
// Use this for batch operations where the caller aggregates results.
func (a *Amadeus) ResolveDMailResult(ctx context.Context, name string, action string, reason string) (ResolveOutput, error) {
	_, resolution, span, err := a.resolveDMailCore(ctx, name, action, reason)
	defer span.End()
	if err != nil {
		return ResolveOutput{}, err
	}
	return ResolveOutput{
		Name:       name,
		Status:     resolution.Status,
		Action:     action,
		ResolvedAt: resolution.ResolvedAt.Format(time.RFC3339),
	}, nil
}

// ResolveDMailJSON resolves a D-Mail and writes the result as JSON to DataOut.
func (a *Amadeus) ResolveDMailJSON(ctx context.Context, name string, action string, reason string) error {
	result, err := a.ResolveDMailResult(ctx, name, action, reason)
	if err != nil {
		return err
	}
	return a.writeDataJSON(result)
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
		resolutions, err := a.Store.LoadResolutions()
		if err != nil {
			return fmt.Errorf("load resolutions: %w", err)
		}
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
			status := string(RouteDMail(d.Severity))
			if res, ok := resolutions[d.Name]; ok {
				status = res.Status
			}
			a.dataOut("  %s  %s %-10s %s",
				d.Name,
				severityTag,
				status,
				d.Description)
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

// dmailJSONView is a JSON-specific view that merges a DMail with its Resolution status.
type dmailJSONView struct {
	Name        string            `json:"name"`
	Kind        DMailKind         `json:"kind"`
	Description string            `json:"description"`
	Issues      []string          `json:"issues,omitempty"`
	Severity    Severity          `json:"severity,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Status      string            `json:"status"`
	ResolvedAt  *time.Time        `json:"resolved_at,omitempty"`
	Reason      string            `json:"reason,omitempty"`
}

// PrintLogJSON writes the history and D-Mail log as JSON to DataOut.
func (a *Amadeus) PrintLogJSON() error {
	history, err := a.Store.LoadHistory()
	if err != nil {
		return fmt.Errorf("load history: %w", err)
	}
	dmails, err := a.Store.LoadAllDMails()
	if err != nil {
		return fmt.Errorf("load dmails: %w", err)
	}
	resolutions, err := a.Store.LoadResolutions()
	if err != nil {
		return fmt.Errorf("load resolutions: %w", err)
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
		status := string(RouteDMail(d.Severity))
		var resolvedAt *time.Time
		var reason string
		if res, ok := resolutions[d.Name]; ok {
			status = res.Status
			resolvedAt = res.ResolvedAt
			reason = res.Reason
		}
		views[i] = dmailJSONView{
			Name:        d.Name,
			Kind:        d.Kind,
			Description: d.Description,
			Issues:      d.Issues,
			Severity:    d.Severity,
			Metadata:    d.Metadata,
			Status:      status,
			ResolvedAt:  resolvedAt,
			Reason:      reason,
		}
	}

	output := struct {
		History  []CheckResult    `json:"history"`
		DMails   []dmailJSONView  `json:"dmails"`
		Consumed []ConsumedRecord `json:"consumed"`
	}{
		History:  history,
		DMails:   views,
		Consumed: consumed,
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
