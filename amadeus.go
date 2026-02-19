package amadeus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Amadeus is the main orchestrator that wires Phase 1 (ReadingSteiner),
// Phase 2 (DivergenceMeter via Claude), and Phase 3 (D-Mail generation).
type Amadeus struct {
	Config        Config
	Store         *StateStore
	Git           *GitClient
	Claude        *ClaudeClient
	Logger        *Logger
	CheckCount    int  // number of diff checks since last full check
	ForceFullNext bool // set when a divergence jump defers a full scan to the next run
}

// CheckOptions controls how RunCheck operates.
type CheckOptions struct {
	Full   bool
	DryRun bool
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
	previous, err := a.Store.LoadLatest()
	if err != nil {
		return fmt.Errorf("load previous state: %w", err)
	}

	// Restore persisted state
	a.CheckCount = previous.CheckCountSinceFull
	a.ForceFullNext = previous.ForceFullNext

	fullCheck := a.ShouldFullCheck(opts.Full)
	if a.ForceFullNext {
		a.Logger.Info("Full scan triggered by previous divergence jump")
		a.ForceFullNext = false // consumed
	}

	rs := &ReadingSteiner{Git: a.Git}
	var report ShiftReport

	if fullCheck {
		report, err = rs.DetectShiftFull(a.Git.Dir)
		if err != nil {
			return fmt.Errorf("phase 1 (full): %w", err)
		}
	} else {
		sinceCommit := previous.Commit
		if sinceCommit == "" {
			fullCheck = true
			report, err = rs.DetectShiftFull(a.Git.Dir)
			if err != nil {
				return fmt.Errorf("phase 1 (first run): %w", err)
			}
		} else {
			report, err = rs.DetectShift(sinceCommit)
			if err != nil {
				return fmt.Errorf("phase 1 (diff): %w", err)
			}
		}
	}

	if !report.Significant {
		a.Logger.Info("Reading Steiner: no significant shift detected")
		currentCommit, err := a.Git.CurrentCommit()
		if err != nil {
			return fmt.Errorf("get current commit: %w", err)
		}
		a.AdvanceCheckCount(fullCheck)
		if err := a.SaveCheckState(currentCommit, previous, time.Now().UTC()); err != nil {
			return fmt.Errorf("save check state: %w", err)
		}
		return nil
	}

	a.Logger.Info("Reading Steiner: %d PRs merged since last check", len(report.MergedPRs))
	for _, pr := range report.MergedPRs {
		a.Logger.Info("  %s %s", pr.Number, pr.Title)
	}

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
		return fmt.Errorf("phase 2 (build prompt): %w", err)
	}

	if opts.DryRun {
		fmt.Println(prompt)
		return nil
	}

	rawResp, err := a.Claude.Run(ctx, prompt)
	if err != nil {
		return fmt.Errorf("phase 2 (claude): %w", err)
	}

	claudeResp, err := ParseClaudeResponse(rawResp)
	if err != nil {
		return fmt.Errorf("phase 2 (parse): %w", err)
	}

	meter := &DivergenceMeter{Config: a.Config}
	meterResult := meter.ProcessResponse(claudeResp)

	// Defer full scan to next run on large divergence jump
	if !fullCheck && a.ShouldPromoteToFull(previous.Divergence, meterResult.Divergence.Value) {
		a.Logger.Info("Divergence jump detected (%.2f → %.2f), next run will trigger full calibration",
			previous.Divergence, meterResult.Divergence.Value)
		a.FlagForceFullNext()
	}

	currentCommit, err := a.Git.CurrentCommit()
	if err != nil {
		return fmt.Errorf("get current commit: %w", err)
	}
	now := time.Now().UTC()

	var dmails []DMail
	for _, candidate := range meterResult.DMailCandidates {
		id, err := a.Store.NextDMailID()
		if err != nil {
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
			return fmt.Errorf("phase 3 (save dmail): %w", err)
		}
		dmails = append(dmails, dmail)
	}

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

	a.PrintCheckOutput(result, dmails, previous.Divergence)

	return nil
}

// PrintCheckOutput renders the CLI display for a completed check.
func (a *Amadeus) PrintCheckOutput(result CheckResult, dmails []DMail, previousDivergence float64) {
	a.Logger.Info("")
	a.Logger.Info("Divergence: %s (%s)",
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
			a.Logger.Info("  %-22s %s — %s",
				axisNames[axis]+":",
				FormatDivergence(contribution),
				score.Details)
		}
	}

	if len(dmails) > 0 {
		a.Logger.Info("")
		a.Logger.Info("D-Mails:")
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
			a.Logger.Info("  %s %s %s → %s to %s",
				prefix, d.ID, d.Summary, status, string(d.Target))
		}
		if pending > 0 {
			a.Logger.Info("")
			a.Logger.Info("%d pending. Run `amadeus resolve <id> --approve` or `--reject`", pending)
		}
	}
}

// ShouldPromoteToFull returns true when the divergence jump between the
// previous and current values exceeds the configured on_divergence_jump threshold.
// This triggers an automatic recalibration (baseline reset).
func (a *Amadeus) ShouldPromoteToFull(previousDivergence, currentDivergence float64) bool {
	delta := currentDivergence - previousDivergence
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
