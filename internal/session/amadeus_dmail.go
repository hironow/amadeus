package session

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness"
	"github.com/hironow/amadeus/internal/platform"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// consumeInbox runs Phase 0: scans the inbox for inbound D-Mails and emits
// inbox-consumed events. Returns the consumed D-Mails for downstream use
// (e.g., extracting feedback_round for circuit-breaker in generateDMails).
func (a *Amadeus) consumeInbox(ctx context.Context, quiet bool) ([]domain.DMail, error) {
	span := trace.SpanFromContext(ctx)

	consumed, scanErr := a.Store.ScanInbox(ctx)
	if scanErr != nil {
		return nil, fmt.Errorf("scan inbox: %w", scanErr)
	}
	if len(consumed) == 0 {
		return nil, nil
	}
	if !quiet {
		a.Logger.Info("Consumed %d report(s) from inbox", len(consumed))
	}
	span.AddEvent("inbox.consumed", trace.WithAttributes(
		attribute.Int("inbox.count", len(consumed)),
	))
	now := time.Now().UTC()
	for _, d := range consumed {
		domain.LogBanner(a.Logger, domain.BannerRecv, string(d.Kind), d.Name, d.Description)
		if err := a.Emitter.EmitInboxConsumed(domain.InboxConsumedData{
			Name:   d.Name,
			Kind:   d.Kind,
			Source: d.Name + ".md",
		}, now); err != nil {
			return nil, fmt.Errorf("emit inbox consumed: %w", err)
		}
	}
	return consumed, nil
}

// generateDMails runs Phase 3: creates D-Mail entities from meter candidates,
// validates them, and emits dmail-generated events.
// This produces KindImplFeedback and/or KindDesignFeedback based on divergence
// scoring (ClassifyByAxes + ResolveFeedbackKinds). Works with or without --base.
// inboxDMails carries consumed inbox D-Mails for feedback_round propagation (may be nil).
func (a *Amadeus) generateDMails(ctx context.Context, meterResult domain.MeterResult, inboxDMails []domain.DMail, now time.Time) ([]domain.DMail, error) {
	_, span3 := platform.Tracer.Start(ctx, "phase.dmail_generation", // nosemgrep: adr0003-otel-span-without-defer-end -- End() called per branch [permanent]
		trace.WithAttributes(
			attribute.Int("phase.number", 3),
			attribute.String("phase.name", "dmail_generation"),
		),
	)
	var dmails []domain.DMail
	quantitative := domain.ClassifyByAxes(meterResult.Divergence.Axes, a.Config.Weights)

	// S02: Extract feedback_round from the triggering report D-Mail (not all inbox D-Mails).
	// This scopes the round counter to the causal thread, not the entire inbox batch.
	// S01: Also extract Wave reference for propagation to feedback D-Mails.
	// Only propagate Wave when exactly one report is in the batch to avoid
	// mis-threading feedback across different waves.
	triggerRound := 0
	var triggerWave *domain.WaveReference
	triggerCorrection := domain.CorrectionMetadata{}
	wavedReportCount := 0
	for _, d := range inboxDMails {
		if d.Kind == domain.KindReport {
			if r := domain.FeedbackRound(d); r > triggerRound {
				triggerRound = r
			}
			if d.Wave != nil {
				wavedReportCount++
				triggerWave = d.Wave
			}
			if meta := domain.CorrectionMetadataFromMap(d.Metadata); meta.IsImprovement() && meta.HasSupportedVocabulary() {
				triggerCorrection = meta
			}
		}
	}
	// Only propagate wave when exactly one waved report in the batch.
	// Multiple waved reports (even same wave ID, different steps) = ambiguous,
	// so drop to avoid mis-threading feedback to the wrong step.
	if wavedReportCount != 1 {
		triggerWave = nil
	}
	nextRound := triggerRound + 1

	// S02: Circuit-breaker — if feedback rounds exhausted, generate convergence instead.
	if triggerRound >= domain.MaxFeedbackRounds {
		a.Logger.Warn("feedback round %d >= max %d — suppressing feedback, convergence detection (Phase 4) will escalate", triggerRound, domain.MaxFeedbackRounds)
		span3.End()
		// Return empty (not nil) so Phase 4 convergence detection still runs.
		return dmails, nil
	}

	for _, candidate := range meterResult.DMailCandidates {
		kinds := domain.ResolveFeedbackKinds(candidate.Category, quantitative)
		for _, kind := range kinds {
			name, err := a.Store.NextDMailName(kind)
			if err != nil {
				span3.End()
				return nil, fmt.Errorf("phase 3 (dmail name): %w", err)
			}
			// #110: Sanitize targets to prevent self-referencing routing loops.
			sanitized := domain.SanitizeTargets("amadeus", kind, candidate.Targets)
			// S02: Enforce required targets (design-feedback → sightjack, impl-feedback → paintress).
			for _, req := range domain.RequiredTargets(kind) {
				if !slices.Contains(sanitized, req) {
					sanitized = append(sanitized, req)
				}
			}
			correctionMeta := dmailCorrectionMetadata(candidate, kind, name, meterResult.Divergence.Severity, triggerWave, triggerRound, triggerCorrection, a.Policy, span3)
			dmail := domain.DMail{
				SchemaVersion: domain.DMailSchemaVersion,
				Name:          name,
				Kind:          kind,
				Description:   candidate.Description,
				Issues:        candidate.Issues,
				Severity:      meterResult.Divergence.Severity,
				Action:        domain.DMailAction(correctionMeta.CorrectiveAction),
				Targets:       sanitized,
				Wave:          triggerWave, // S01: propagate wave reference from triggering report
				Metadata: correctionMeta.Apply(map[string]string{
					"created_at":     now.Format(time.RFC3339),
					"feedback_round": strconv.Itoa(nextRound),
				}),
				Body: candidate.Detail + formatADRViolations(meterResult),
			}
			if dmail.Action == "" {
				dmail.Action = domain.DefaultDMailAction(meterResult.Divergence.Severity)
			}
			if errs := harness.ValidateDMail(dmail); len(errs) > 0 {
				a.Logger.Warn("skipping invalid %s dmail %s: %v", kind, name, errs)
				continue
			}
			domain.LogBanner(a.Logger, domain.BannerSend, string(dmail.Kind), dmail.Name, dmail.Description)
			if err := a.Emitter.EmitDMailGenerated(dmail, now); err != nil {
				span3.End()
				return nil, fmt.Errorf("phase 3 (emit dmail): %w", err)
			}
			span3.AddEvent("dmail.created", trace.WithAttributes(
				attribute.String("dmail.name", platform.SanitizeUTF8(dmail.Name)),
				attribute.String("dmail.kind", platform.SanitizeUTF8(string(dmail.Kind))),
				attribute.String("dmail.severity", platform.SanitizeUTF8(string(dmail.Severity))),
			))
			dmails = append(dmails, dmail)
		}
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
	allDMails = domain.FilterByTTL(allDMails, now)
	convergenceAlerts := a.Config.DetectConvergence(allDMails, now)
	for _, alert := range convergenceAlerts {
		if err := a.Emitter.EmitConvergenceDetected(alert, now); err != nil {
			return nil, nil, fmt.Errorf("phase 4 (emit convergence event): %w", err)
		}
	}
	saved, saveErr := a.saveConvergenceDMails(convergenceAlerts)
	if saveErr != nil {
		return convergenceAlerts, nil, saveErr
	}
	return convergenceAlerts, saved, nil
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
		domain.LogBanner(a.Logger, domain.BannerSend, string(cd.Kind), cd.Name, cd.Description)
		if err := a.Emitter.EmitDMailGenerated(cd, time.Now().UTC()); err != nil {
			return saved, fmt.Errorf("emit convergence dmail %s: %w", cdName, err)
		}
		saved = append(saved, cd)
	}
	return saved, nil
}

// formatADRViolations appends a "Violated ADRs" section to D-Mail body
// when per-ADR alignment data is available and has violations.
func formatADRViolations(meterResult domain.MeterResult) string {
	if len(meterResult.Divergence.ADRAlignment) == 0 {
		return ""
	}
	section := domain.FormatViolatedADRsSection(meterResult.Divergence.ADRAlignment, nil, 70)
	if section == "" {
		return ""
	}
	return "\n\n" + section
}

func failureTypeForCandidate(candidate domain.ClaudeDMailCandidate) domain.FailureType {
	switch strings.ToLower(candidate.Category) {
	case "design":
		return domain.FailureTypeScopeViolation
	case "implementation":
		return domain.FailureTypeExecutionFailure
	default:
		return domain.FailureTypeNone
	}
}

func dmailCorrectionMetadata(candidate domain.ClaudeDMailCandidate, kind domain.DMailKind, name string, severity domain.Severity, wave *domain.WaveReference, triggerRound int, trigger domain.CorrectionMetadata, policy domain.RoutingPolicy, span trace.Span) domain.CorrectionMetadata {
	recurrenceCount := triggerRound
	routingHistory := []string(nil)
	ownerHistory := []string(nil)
	meta := domain.CorrectionMetadata{
		SchemaVersion:   domain.ImprovementSchemaVersion,
		FailureType:     failureTypeForCandidate(candidate),
		Severity:        domain.NormalizeSeverity(severity),
		SecondaryType:   strings.ToLower(candidate.Category),
		RoutingHistory:  routingHistory,
		OwnerHistory:    ownerHistory,
		RecurrenceCount: recurrenceCount,
		CorrelationID:   correlationIDForDMail(name, wave),
		TraceID:         span.SpanContext().TraceID().String(),
		Outcome:         domain.ImprovementOutcomePending,
	}
	if trigger.IsImprovement() {
		meta.SchemaVersion = trigger.ConsumerSchemaVersion()
		if trigger.FailureType != "" {
			meta.FailureType = trigger.FailureType
		}
		if trigger.SecondaryType != "" {
			meta.SecondaryType = trigger.SecondaryType
		}
		if trigger.CorrelationID != "" {
			meta.CorrelationID = trigger.CorrelationID
		}
		meta.RoutingHistory = append([]string(nil), trigger.RoutingHistory...)
		meta.OwnerHistory = append([]string(nil), trigger.OwnerHistory...)
		if len(meta.RoutingHistory) == 0 && trigger.RoutingMode != "" {
			meta.RoutingHistory = domain.AppendImprovementHistory(meta.RoutingHistory, string(domain.NormalizeRoutingMode(trigger.RoutingMode)))
		}
		if len(meta.OwnerHistory) == 0 && trigger.TargetAgent != "" {
			meta.OwnerHistory = domain.AppendImprovementHistory(meta.OwnerHistory, trigger.TargetAgent)
		}
		recurrenceCount = trigger.RecurrenceCount + 1
		meta.RecurrenceCount = recurrenceCount
		meta.Outcome = domain.ImprovementOutcomeFailedAgain
	}
	decision := harness.DetermineCorrectionDecision(kind, severity, domain.DMailAction(candidate.Action), meta.FailureType, recurrenceCount, trigger, currentProviderState(), policy)
	meta.RoutingMode = decision.RoutingMode
	meta.TargetAgent = decision.TargetAgent
	if decision.RoutingMode != "" {
		meta.RoutingHistory = domain.AppendImprovementHistory(meta.RoutingHistory, string(domain.NormalizeRoutingMode(decision.RoutingMode)))
	}
	if decision.TargetAgent != "" {
		meta.OwnerHistory = domain.AppendImprovementHistory(meta.OwnerHistory, decision.TargetAgent)
	}
	meta.CorrectiveAction = string(decision.Action)
	meta.RetryAllowed = decision.RetryAllowed
	meta.EscalationReason = decision.EscalationReason
	if decision.RoutingMode == domain.RoutingModeEscalate {
		meta.Severity = domain.SeverityHigh
	}
	meta.Outcome = correctionOutcome(decision.Action, trigger)
	return meta
}

func correctionOutcome(action domain.DMailAction, trigger domain.CorrectionMetadata) domain.ImprovementOutcome {
	if action == domain.ActionEscalate {
		return domain.ImprovementOutcomeEscalated
	}
	if trigger.IsImprovement() {
		return domain.ImprovementOutcomeFailedAgain
	}
	return domain.ImprovementOutcomePending
}

func correlationIDForDMail(name string, wave *domain.WaveReference) string {
	if wave != nil && wave.ID != "" {
		if wave.Step != "" {
			return wave.ID + ":" + wave.Step
		}
		return wave.ID
	}
	return name
}
