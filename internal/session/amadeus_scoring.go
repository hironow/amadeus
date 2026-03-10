package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// detectShift runs Phase 1: ReadingSteiner shift detection.
// Returns the shift report, whether a full check was performed, and any error.
func (a *Amadeus) detectShift(ctx context.Context, previous domain.CheckResult, fullMode bool, quiet bool) (ShiftReport, bool, error) {
	fullCheck := a.State.ShouldFullCheck(fullMode)
	if a.State.ForceFullNext() {
		if !quiet {
			a.Logger.Info("Full scan triggered by previous divergence jump")
		}
		a.State.SetForceFullNext(false) // consumed
	}

	rs := &ReadingSteiner{Git: a.Git}
	var report ShiftReport
	var err error

	_, span1 := platform.Tracer.Start(ctx, "phase.reading_steiner", // nosemgrep: adr0003-otel-span-without-defer-end -- End() called per branch [permanent]
		trace.WithAttributes(
			attribute.Int("phase.number", 1),
			attribute.String("phase.name", "reading_steiner"),
		),
	)
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

	// Enrich with GitHub PR review data (graceful: skip if gh unavailable)
	if len(report.MergedPRs) > 0 {
		gh := &GHClient{Dir: a.RepoDir}
		reviews := gh.FetchPRReviews(report.MergedPRs)
		if len(reviews) > 0 {
			report.PRReviews = reviews
			span1.AddEvent("pr_reviews.fetched", trace.WithAttributes(
				attribute.Int("pr_reviews.count", len(reviews)),
			))
		}
	}
	span1.End()

	return report, fullCheck, nil
}

// buildCheckPrompt runs Phase 2a: collects ADRs, DoDs, and dependency map,
// then builds the appropriate Claude prompt (full or diff).
func (a *Amadeus) buildCheckPrompt(ctx context.Context, report ShiftReport, fullCheck bool, previous domain.CheckResult, quiet bool) (string, error) {
	repoRoot := a.RepoDir
	allADRs, adrErr := CollectADRs(ctx, repoRoot)
	if adrErr != nil && !quiet {
		a.Logger.Info("Warning: failed to collect ADRs: %v", adrErr)
	}
	allDoDs, dodErr := CollectDoDs(ctx, repoRoot)
	if dodErr != nil && !quiet {
		a.Logger.Info("Warning: failed to collect DoDs: %v", dodErr)
	}
	depMap, depErr := CollectDependencyMap(repoRoot)
	if depErr != nil && !quiet {
		a.Logger.Info("Warning: failed to collect dependency map: %v", depErr)
	}

	if fullCheck {
		return platform.BuildFullCheckPrompt(a.Config.ConfigLang(), domain.FullCheckParams{
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
	return platform.BuildDiffCheckPrompt(a.Config.ConfigLang(), domain.DiffCheckParams{
		PreviousScores:  string(prevJSON),
		PRDiffs:         report.Diff,
		RelevantADRs:    allADRs,
		LinkedDoDs:      linkedDoDs,
		LinkedIssueIDs:  strings.Join(issueIDs, ", "),
		PRReviewSummary: domain.FormatPRReviewSummary(report.PRReviews),
	})
}

// runDivergenceMeter runs Phase 2b: executes Claude, parses the response,
// scores with DivergenceMeter, and handles divergence jump detection.
func (a *Amadeus) runDivergenceMeter(ctx context.Context, prompt string, fullCheck bool, previous domain.CheckResult, quiet bool) (domain.MeterResult, error) {
	_, span2 := platform.Tracer.Start(ctx, "phase.divergence_meter", // nosemgrep: adr0003-otel-span-without-defer-end -- End() called per branch [permanent]
		trace.WithAttributes(
			attribute.Int("phase.number", 2),
			attribute.String("phase.name", "divergence_meter"),
		),
	)

	// claude.invoke span wraps the Claude CLI execution with GenAI semconv attributes.
	model := a.ClaudeModel
	timeoutSec := 0
	if deadline, ok := ctx.Deadline(); ok {
		timeoutSec = int(time.Until(deadline).Seconds())
		if timeoutSec < 0 {
			timeoutSec = 0
		}
	}
	invokeCtx, invokeSpan := platform.Tracer.Start(ctx, "claude.invoke", // nosemgrep: adr0003-otel-span-without-defer-end — End() called explicitly after Run() [permanent]
		trace.WithAttributes(
			append([]attribute.KeyValue{
				attribute.String("claude.model", model),
				attribute.Int("claude.timeout_sec", timeoutSec),
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
	if !fullCheck && a.State.ShouldPromoteToFull(previous.Divergence, meterResult.Divergence.Value) {
		span2.AddEvent("divergence.jump", trace.WithAttributes(
			attribute.Float64("divergence.previous", previous.Divergence),
			attribute.Float64("divergence.current", meterResult.Divergence.Value),
		))
		if !quiet {
			a.Logger.Info("Divergence jump detected (%.2f -> %.2f), next run will trigger full calibration",
				previous.Divergence, meterResult.Divergence.Value)
		}
		if err := a.Emitter.EmitForceFullNextSet(previous.Divergence, meterResult.Divergence.Value, time.Now().UTC()); err != nil {
			span2.End()
			return domain.MeterResult{}, fmt.Errorf("emit force_full_next: %w", err)
		}
	}
	span2.End()

	return meterResult, nil
}
