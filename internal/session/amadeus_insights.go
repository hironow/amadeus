package session

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
)

// writeDivergenceInsight writes a divergence insight entry after scoring.
// Fails silently (log warning) to avoid breaking the check pipeline.
func (a *Amadeus) writeDivergenceInsight(result domain.DivergenceResult, sessionID string, commitRange string, reasoning string) {
	if a.Insights == nil {
		return
	}

	// Build axis details for the "why" field
	whyParts := highScoringAxisDetails(result.Axes)
	why := "No high-scoring axes"
	if len(whyParts) > 0 {
		why = strings.Join(whyParts, "; ")
	}

	how := "Focus remediation on highest-scoring axis"
	if reasoning != "" {
		how = reasoning
	}

	entry := domain.InsightEntry{
		Title:       fmt.Sprintf("divergence-%s", sessionID),
		What:        fmt.Sprintf("Divergence score: %f, severity: %s", result.Value, result.Severity),
		Why:         why,
		How:         how,
		When:        fmt.Sprintf("Check on commits %s", commitRange),
		Who:         fmt.Sprintf("amadeus run (session-%s)", sessionID),
		Constraints: "Scores relative to configured weights",
		Extra:       divergenceExtra(result),
	}

	if err := a.Insights.Append("divergence.md", "divergence", "amadeus", entry); err != nil {
		a.Logger.Warn("insight write (divergence): %v", err)
	}
}

// writeConvergenceInsight writes a convergence insight entry for a HIGH severity alert.
// Fails silently (log warning) to avoid breaking the check pipeline.
func (a *Amadeus) writeConvergenceInsight(alert domain.ConvergenceAlert, sessionID string) {
	if a.Insights == nil {
		return
	}

	if alert.Severity != domain.SeverityHigh {
		return
	}

	why := "Multiple feedback signals targeting same area indicates structural issue"
	if len(alert.Descriptions) > 0 {
		var descs []string
		for _, name := range alert.DMails {
			if desc, ok := alert.Descriptions[name]; ok {
				descs = append(descs, desc)
			}
		}
		if len(descs) > 0 {
			why = fmt.Sprintf("Converging feedback: %s", strings.Join(descs, "; "))
		}
	}

	entry := domain.InsightEntry{
		Title:       fmt.Sprintf("convergence-%s", alert.Target),
		What:        fmt.Sprintf("World line convergence on %s: %d D-Mails in %d days", alert.Target, alert.Count, alert.Window),
		Why:         why,
		How:         "Investigate for shared root cause",
		When:        fmt.Sprintf("When %d D-Mails target same area within window", alert.Count),
		Who:         fmt.Sprintf("amadeus convergence detector (session-%s)", sessionID),
		Constraints: "Escalation threshold",
		Extra: map[string]string{
			"related-dmails": strings.Join(alert.DMails, ", "),
		},
	}

	if err := a.Insights.Append("convergence.md", "convergence", "amadeus", entry); err != nil {
		a.Logger.Warn("insight write (convergence): %v", err)
	}
}

func (a *Amadeus) writeImprovementOutcomeInsight(inboxDMails []domain.DMail, sessionID string, dmailCount int) {
	if a.Insights == nil {
		return
	}
	meta := latestImprovementMetadata(inboxDMails)
	if !meta.IsImprovement() || !meta.HasSupportedVocabulary() {
		return
	}
	outcome := domain.ImprovementOutcomeResolved
	if dmailCount > 0 {
		outcome = domain.ImprovementOutcomeFailedAgain
	}
	entry := improvementOutcomeInsight(meta, outcome, sessionID)
	if err := a.Insights.Append("improvement-loop.md", "improvement-loop", "amadeus", entry); err != nil {
		a.Logger.Warn("insight write (improvement-loop): %v", err)
	}
}

func latestImprovementMetadata(dmails []domain.DMail) domain.CorrectionMetadata {
	for i := len(dmails) - 1; i >= 0; i-- {
		if dmails[i].Kind != domain.KindReport {
			continue
		}
		meta := domain.CorrectionMetadataFromMap(dmails[i].Metadata)
		if meta.IsImprovement() && meta.HasSupportedVocabulary() {
			return meta
		}
	}
	return domain.CorrectionMetadata{}
}

func improvementOutcomeInsight(meta domain.CorrectionMetadata, outcome domain.ImprovementOutcome, sessionID string) domain.InsightEntry {
	meta.SchemaVersion = domain.ImprovementSchemaVersion
	meta.Outcome = outcome
	title := fmt.Sprintf("improvement-%s-%s", fallbackImprovementTitle(meta.CorrelationID), outcome)
	what := fmt.Sprintf("Corrective rerun for %s ended as %s", fallbackFailureType(meta), outcome)
	why := "Amadeus classified the next run using normalized corrective metadata from the inbound report"
	how := "Use this outcome to compare before/after reruns and refine corrective routing"
	if outcome == domain.ImprovementOutcomeResolved {
		how = "Use this resolution to confirm the rerun fixed the prior corrective thread"
	}
	entry := domain.InsightEntry{
		Title:       title,
		What:        what,
		Why:         why,
		How:         how,
		When:        fmt.Sprintf("Check on commit %s", sessionID),
		Who:         fmt.Sprintf("amadeus improvement classifier (session-%s)", sessionID),
		Constraints: fmt.Sprintf("improvement schema %s", meta.SchemaVersion),
		Extra: map[string]string{
			"outcome":      string(outcome),
			"failure-type": fallbackFailureType(meta),
		},
	}
	if meta.CorrelationID != "" {
		entry.Extra["correlation-id"] = meta.CorrelationID
	}
	if meta.TraceID != "" {
		entry.Extra["trace-id"] = meta.TraceID
	}
	if meta.CorrectiveAction != "" {
		entry.Extra["corrective-action"] = meta.CorrectiveAction
	}
	if meta.Severity != "" {
		entry.Extra["severity"] = string(domain.NormalizeSeverity(meta.Severity))
	}
	if meta.RetryAllowed != nil {
		entry.Extra["retry-allowed"] = strconv.FormatBool(*meta.RetryAllowed)
	}
	if meta.EscalationReason != "" {
		entry.Extra["escalation-reason"] = meta.EscalationReason
	}
	return entry
}

func fallbackImprovementTitle(correlationID string) string {
	if correlationID == "" {
		return "uncorrelated"
	}
	return correlationID
}

func fallbackFailureType(meta domain.CorrectionMetadata) string {
	if meta.FailureType == "" {
		return string(domain.FailureTypeNone)
	}
	return string(meta.FailureType)
}

// highScoringAxisDetails returns detail strings for axes with score >= 50.
func highScoringAxisDetails(axes map[domain.Axis]domain.AxisScore) []string {
	var parts []string
	// Sort keys for deterministic output
	keys := make([]string, 0, len(axes))
	for k := range axes {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)

	for _, k := range keys {
		axis := domain.Axis(k)
		as := axes[axis]
		if as.Score >= 50 {
			parts = append(parts, fmt.Sprintf("%s=%d (%s)", axis, as.Score, as.Details))
		}
	}
	return parts
}

// divergenceExtra builds the Extra map for a divergence insight entry.
func divergenceExtra(result domain.DivergenceResult) map[string]string {
	// Format axis scores
	var axisParts []string
	keys := make([]string, 0, len(result.Axes))
	for k := range result.Axes {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	for _, k := range keys {
		axis := domain.Axis(k)
		as := result.Axes[axis]
		axisParts = append(axisParts, fmt.Sprintf("%s=%d", axis, as.Score))
	}

	return map[string]string{
		"axis-scores": strings.Join(axisParts, ", "),
	}
}
