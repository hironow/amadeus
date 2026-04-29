package session

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

type improvementSignalSurface string

const (
	improvementSurfaceFeedback improvementSignalSurface = "feedback"
	improvementSurfaceCI       improvementSignalSurface = "ci"
	improvementSurfacePR       improvementSignalSurface = "pr"
	improvementSurfaceScorer   improvementSignalSurface = "scorer"
	improvementSurfaceTrace    improvementSignalSurface = "trace"
)

type ImprovementFeedbackQuery struct { // nosemgrep: structure.multiple-exported-structs-go,structure.exported-struct-and-interface-go -- improvement collector family (ImprovementFeedbackQuery/ImprovementFeedbackRow/ImprovementFeedbackSource/ImprovementCollector) is a cohesive query/source port schema; co-locates with ImprovementFeedbackSource interface as query parameter [permanent]
	ProjectID     string
	CreatedAfter  time.Time
	AfterFeedback string
	Limit         int
	FeedbackTypes []string
}

type ImprovementFeedbackRow struct { // nosemgrep: structure.multiple-exported-structs-go,structure.exported-struct-and-interface-go -- improvement collector family cohesive set; see ImprovementFeedbackQuery [permanent]
	ID           string
	ProjectID    string
	WeaveRef     string
	FeedbackType string
	CreatedAt    time.Time
	Payload      map[string]any
}

type ImprovementFeedbackSource interface {
	QueryFeedback(ctx context.Context, query ImprovementFeedbackQuery) ([]ImprovementFeedbackRow, error)
}

type ImprovementCollector struct {
	ProjectID            string
	Source               ImprovementFeedbackSource
	Store                *SQLiteImprovementCollectorStore
	Ledger               *InsightWriter
	Logger               domain.Logger
	QueryLimit           int
	AllowedFeedbackTypes []string
}

type normalizedImprovementFeedbackRecord struct {
	Entry  domain.InsightEntry
	Signal NormalizedImprovementSignal
}

func (c *ImprovementCollector) PollOnce(ctx context.Context, limit int) (int, error) {
	if c == nil {
		return 0, nil
	}
	if c.Source == nil {
		return 0, fmt.Errorf("improvement collector: source is required")
	}
	if c.Store == nil {
		return 0, fmt.Errorf("improvement collector: store is required")
	}
	if c.Ledger == nil {
		return 0, fmt.Errorf("improvement collector: ledger is required")
	}
	cursor, err := c.Store.LoadCursor(ctx)
	if err != nil {
		return 0, err
	}
	rows, err := c.Source.QueryFeedback(ctx, ImprovementFeedbackQuery{
		ProjectID:     c.ProjectID,
		CreatedAfter:  cursor.CreatedAt,
		AfterFeedback: cursor.FeedbackID,
		Limit:         improvementCollectorQueryLimit(limit, c.QueryLimit),
		FeedbackTypes: c.AllowedFeedbackTypes,
	})
	if err != nil {
		return 0, fmt.Errorf("improvement collector: query feedback: %w", err)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].CreatedAt.Before(rows[j].CreatedAt)
	})

	processed := 0
	for _, row := range rows {
		if !improvementFeedbackTypeAllowed(c.AllowedFeedbackTypes, row.FeedbackType) {
			continue
		}
		record := normalizeImprovementFeedbackRecord(row)
		applied, err := c.Store.ApplyFeedback(ctx, row, record.Signal, func() error {
			return c.Ledger.Append("improvement-loop.md", "improvement-loop", "amadeus", record.Entry)
		})
		if err != nil {
			return processed, err
		}
		if applied {
			processed++
		}
	}
	return processed, nil
}

func improvementCollectorQueryLimit(runtimeLimit, configuredLimit int) int {
	if runtimeLimit > 0 {
		return runtimeLimit
	}
	if configuredLimit > 0 {
		return configuredLimit
	}
	return 100
}

func improvementFeedbackTypeAllowed(allowed []string, feedbackType string) bool {
	if len(allowed) == 0 {
		return true
	}
	normalized := normalizeImprovementToken(feedbackType)
	for _, candidate := range allowed {
		if normalizeImprovementToken(candidate) == normalized {
			return true
		}
	}
	return false
}

func normalizeImprovementFeedback(row ImprovementFeedbackRow) domain.InsightEntry {
	return normalizeImprovementFeedbackRecord(row).Entry
}

func normalizeImprovementFeedbackRecord(row ImprovementFeedbackRow) normalizedImprovementFeedbackRecord {
	surface := detectImprovementSurface(row)
	meta := domain.CorrectionMetadata{
		SchemaVersion: domain.ImprovementSchemaVersion,
		FailureType:   domain.FailureType(payloadString(row.Payload, "failure_type")),
		Severity:      domain.NormalizeSeverity(domain.Severity(payloadString(row.Payload, "severity"))),
		SecondaryType: payloadString(row.Payload, "secondary_type"),
		TargetAgent:   payloadString(row.Payload, "target_agent"),
		RoutingMode:   domain.NormalizeRoutingMode(domain.RoutingMode(payloadString(row.Payload, "routing_mode"))),
		RoutingHistory: domain.ParseImprovementHistory(
			payloadString(row.Payload, "routing_history"),
		),
		OwnerHistory: domain.ParseImprovementHistory(
			payloadString(row.Payload, "owner_history"),
		),
		CorrelationID: payloadString(row.Payload, "correlation_id"),
		TraceID:       payloadString(row.Payload, "trace_id"),
		Outcome:       domain.NormalizeImprovementOutcome(domain.ImprovementOutcome(payloadString(row.Payload, "outcome"))),
	}
	if action := payloadString(row.Payload, "corrective_action"); action != "" {
		meta.CorrectiveAction = action
	}
	if reason := payloadString(row.Payload, "escalation_reason"); reason != "" {
		meta.EscalationReason = reason
	}
	if recurrence, ok := payloadInt(row.Payload, "recurrence_count"); ok {
		meta.RecurrenceCount = recurrence
	}
	if retryAllowed, ok := payloadBool(row.Payload, "retry_allowed"); ok {
		meta.RetryAllowed = domain.BoolPtr(retryAllowed)
	}
	if meta.SecondaryType == "" && surface != improvementSurfaceFeedback {
		meta.SecondaryType = string(surface)
	}
	if meta.Outcome == "" {
		meta.Outcome = inferImprovementOutcome(surface, row.Payload)
	}
	if meta.FailureType == "" {
		meta.FailureType = inferImprovementFailureType(surface, row.Payload, meta.Outcome)
	}

	ignoredReason := ""
	switch {
	case meta.FailureType == "":
		ignoredReason = "missing-failure-type"
	case meta.Severity != "" && !domain.IsKnownSeverity(meta.Severity):
		ignoredReason = "unsupported-severity"
		meta.Severity = ""
	case meta.Outcome != "" && !domain.IsKnownImprovementOutcome(meta.Outcome):
		ignoredReason = "unsupported-outcome"
		meta.Outcome = ""
	}
	if ignoredReason != "" {
		meta.Outcome = domain.ImprovementOutcomeIgnored
	}

	what := fmt.Sprintf("Normalized Weave %s %s", improvementSurfaceLabel(surface), row.ID)
	how := "Use the normalized feedback to seed the corrective thread ledger"
	if meta.FailureType != "" {
		what = fmt.Sprintf("Normalized Weave %s %s as %s", improvementSurfaceLabel(surface), row.ID, meta.FailureType)
	}
	if meta.Outcome == domain.ImprovementOutcomeIgnored {
		what = fmt.Sprintf("Ignored Weave %s %s", improvementSurfaceLabel(surface), row.ID)
		how = "Fix the feedback payload or routing policy before re-ingesting this signal"
	}

	entry := domain.InsightEntry{
		Title:       fmt.Sprintf("weave-%s-%s", surface, row.ID),
		What:        what,
		Why:         "Weave signals are normalized into the improvement ledger before corrective reruns are classified",
		How:         how,
		When:        fmt.Sprintf("Observed at %s", row.CreatedAt.UTC().Format(time.RFC3339)),
		Who:         "amadeus improvement collector",
		Constraints: fmt.Sprintf("improvement schema %s", domain.ImprovementSchemaVersion),
		Extra: map[string]string{
			"feedback-id":    row.ID,
			"project-id":     row.ProjectID,
			"feedback-type":  row.FeedbackType,
			"weave-ref":      row.WeaveRef,
			"source-surface": string(surface),
		},
	}
	appendImprovementSurfaceExtras(entry.Extra, surface, row.Payload)
	if meta.FailureType != "" {
		entry.Extra["failure-type"] = string(meta.FailureType)
	}
	if meta.Severity != "" {
		entry.Extra["severity"] = string(domain.NormalizeSeverity(meta.Severity))
	}
	if meta.SecondaryType != "" {
		entry.Extra["secondary-type"] = meta.SecondaryType
	}
	if meta.TargetAgent != "" {
		entry.Extra["target-agent"] = meta.TargetAgent
	}
	if len(meta.RoutingHistory) > 0 {
		entry.Extra["routing-history"] = domain.FormatImprovementHistory(meta.RoutingHistory)
	}
	if len(meta.OwnerHistory) > 0 {
		entry.Extra["owner-history"] = domain.FormatImprovementHistory(meta.OwnerHistory)
	}
	if meta.CorrectiveAction != "" {
		entry.Extra["corrective-action"] = meta.CorrectiveAction
	}
	if meta.RoutingMode != "" {
		entry.Extra["routing-mode"] = string(domain.NormalizeRoutingMode(meta.RoutingMode))
	}
	if meta.CorrelationID != "" {
		entry.Extra["correlation-id"] = meta.CorrelationID
	}
	if meta.TraceID != "" {
		entry.Extra["trace-id"] = meta.TraceID
	}
	if meta.Outcome != "" {
		entry.Extra["outcome"] = string(meta.Outcome)
	}
	if meta.RetryAllowed != nil {
		entry.Extra["retry-allowed"] = strconv.FormatBool(*meta.RetryAllowed)
	}
	if meta.EscalationReason != "" {
		entry.Extra["escalation-reason"] = meta.EscalationReason
	}
	if meta.RecurrenceCount > 0 {
		entry.Extra["recurrence-count"] = strconv.Itoa(meta.RecurrenceCount)
	}
	if ignoredReason != "" {
		entry.Extra["ignored-reason"] = ignoredReason
	}
	return normalizedImprovementFeedbackRecord{
		Entry: entry,
		Signal: NormalizedImprovementSignal{
			DedupKey:         improvementFeedbackDedupKey(row),
			FeedbackID:       row.ID,
			ProjectID:        row.ProjectID,
			WeaveRef:         row.WeaveRef,
			FeedbackType:     row.FeedbackType,
			SourceSurface:    string(surface),
			SchemaVersion:    meta.ConsumerSchemaVersion(),
			FailureType:      string(meta.FailureType),
			Severity:         string(domain.NormalizeSeverity(meta.Severity)),
			SecondaryType:    meta.SecondaryType,
			TargetAgent:      meta.TargetAgent,
			RoutingMode:      string(domain.NormalizeRoutingMode(meta.RoutingMode)),
			RoutingHistory:   append([]string(nil), meta.RoutingHistory...),
			OwnerHistory:     append([]string(nil), meta.OwnerHistory...),
			RecurrenceCount:  meta.RecurrenceCount,
			CorrectiveAction: meta.CorrectiveAction,
			RetryAllowed:     meta.RetryAllowed,
			EscalationReason: meta.EscalationReason,
			CorrelationID:    meta.CorrelationID,
			TraceID:          meta.TraceID,
			Outcome:          string(meta.Outcome),
			IgnoredReason:    ignoredReason,
			PayloadJSON:      marshalImprovementPayload(row.Payload),
			CreatedAt:        row.CreatedAt.UTC(),
		},
	}
}

func detectImprovementSurface(row ImprovementFeedbackRow) improvementSignalSurface {
	for _, key := range []string{"source_surface", "surface", "signal_source"} {
		switch normalizeImprovementToken(payloadString(row.Payload, key)) {
		case "ci", "ci_outcome", "ci_result":
			return improvementSurfaceCI
		case "pr", "pr_outcome", "pull_request":
			return improvementSurfacePR
		case "scorer", "scorer_outcome", "score":
			return improvementSurfaceScorer
		case "trace", "trace_outcome":
			return improvementSurfaceTrace
		case "feedback", "weave_feedback":
			return improvementSurfaceFeedback
		}
	}
	switch {
	case payloadString(row.Payload, "ci_status") != "" || payloadString(row.Payload, "workflow_name") != "" || payloadString(row.Payload, "conclusion") != "":
		return improvementSurfaceCI
	case payloadString(row.Payload, "pr_number") != "" || payloadString(row.Payload, "review_decision") != "" || payloadString(row.Payload, "merge_state_status") != "":
		return improvementSurfacePR
	case payloadString(row.Payload, "scorer_verdict") != "" || payloadString(row.Payload, "divergence_severity") != "" || payloadString(row.Payload, "score_name") != "":
		return improvementSurfaceScorer
	case payloadString(row.Payload, "trace_status") != "" || payloadString(row.Payload, "trace_name") != "" || payloadString(row.Payload, "trace_summary") != "":
		return improvementSurfaceTrace
	default:
		return improvementSurfaceFeedback
	}
}

func improvementSurfaceLabel(surface improvementSignalSurface) string {
	switch surface {
	case improvementSurfaceCI:
		return "CI outcome"
	case improvementSurfacePR:
		return "PR outcome"
	case improvementSurfaceScorer:
		return "scorer outcome"
	case improvementSurfaceTrace:
		return "trace outcome"
	default:
		return "feedback"
	}
}

func inferImprovementOutcome(surface improvementSignalSurface, payload map[string]any) domain.ImprovementOutcome {
	statusKeys := map[improvementSignalSurface][]string{
		improvementSurfaceCI:     {"ci_status", "conclusion"},
		improvementSurfacePR:     {"pr_state", "review_decision", "merge_state_status"},
		improvementSurfaceScorer: {"scorer_verdict", "divergence_severity", "score_status"},
		improvementSurfaceTrace:  {"trace_status", "trace_result"},
	}
	for _, key := range statusKeys[surface] {
		token := normalizeImprovementToken(payloadString(payload, key))
		switch token {
		case "success", "succeeded", "passed", "pass", "approved", "merged", "clean", "resolved", "converged", "ok", "completed":
			return domain.ImprovementOutcomeResolved
		case "failure", "failed", "error", "errored", "timed_out", "cancelled", "changes_requested", "blocked", "conflict", "diverged", "drifted", "rejected":
			return domain.ImprovementOutcomeFailedAgain
		}
	}
	return ""
}

func inferImprovementFailureType(surface improvementSignalSurface, payload map[string]any, outcome domain.ImprovementOutcome) domain.FailureType {
	if outcome == "" {
		return ""
	}
	switch surface {
	case improvementSurfaceCI:
		return domain.FailureTypeExecutionFailure
	case improvementSurfacePR, improvementSurfaceScorer:
		return domain.FailureTypeScopeViolation
	case improvementSurfaceTrace:
		errorType := normalizeImprovementToken(payloadString(payload, "error_type"))
		if strings.Contains(errorType, "provider") {
			return domain.FailureTypeProviderFailure
		}
		return domain.FailureTypeExecutionFailure
	default:
		return ""
	}
}

func appendImprovementSurfaceExtras(extra map[string]string, surface improvementSignalSurface, payload map[string]any) {
	switch surface {
	case improvementSurfaceCI:
		appendImprovementPayloadExtra(extra, "ci-status", payload, "ci_status", "conclusion")
		appendImprovementPayloadExtra(extra, "ci-workflow", payload, "workflow_name")
		appendImprovementPayloadExtra(extra, "ci-run-id", payload, "run_id")
		appendImprovementPayloadExtra(extra, "ci-branch", payload, "branch")
	case improvementSurfacePR:
		appendImprovementPayloadExtra(extra, "pr-number", payload, "pr_number")
		appendImprovementPayloadExtra(extra, "pr-state", payload, "pr_state")
		appendImprovementPayloadExtra(extra, "pr-review-decision", payload, "review_decision")
		appendImprovementPayloadExtra(extra, "pr-merge-state-status", payload, "merge_state_status")
		appendImprovementPayloadExtra(extra, "pr-url", payload, "pr_url")
	case improvementSurfaceScorer:
		appendImprovementPayloadExtra(extra, "scorer-verdict", payload, "scorer_verdict", "score_status")
		appendImprovementPayloadExtra(extra, "scorer-name", payload, "score_name")
		appendImprovementPayloadExtra(extra, "scorer-value", payload, "score_value")
		appendImprovementPayloadExtra(extra, "divergence-severity", payload, "divergence_severity")
	case improvementSurfaceTrace:
		appendImprovementPayloadExtra(extra, "trace-status", payload, "trace_status", "trace_result")
		appendImprovementPayloadExtra(extra, "trace-name", payload, "trace_name")
		appendImprovementPayloadExtra(extra, "trace-summary", payload, "trace_summary")
		appendImprovementPayloadExtra(extra, "span-name", payload, "span_name")
	}
}

func appendImprovementPayloadExtra(extra map[string]string, extraKey string, payload map[string]any, payloadKeys ...string) {
	for _, key := range payloadKeys {
		if value := payloadString(payload, key); value != "" {
			extra[extraKey] = value
			return
		}
	}
}

func normalizeImprovementToken(raw string) string {
	token := strings.TrimSpace(strings.ToLower(raw))
	token = strings.ReplaceAll(token, " ", "_")
	token = strings.ReplaceAll(token, "-", "_")
	return token
}

func improvementFeedbackDedupKey(row ImprovementFeedbackRow) string {
	return row.ProjectID + "|" + row.ID + "|" + row.WeaveRef + "|" + row.FeedbackType
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	raw, ok := payload[key]
	if !ok || raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func payloadInt(payload map[string]any, key string) (int, bool) {
	if payload == nil {
		return 0, false
	}
	raw, ok := payload[key]
	if !ok || raw == nil {
		return 0, false
	}
	switch v := raw.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		parsed, err := strconv.Atoi(v)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func payloadBool(payload map[string]any, key string) (bool, bool) {
	if payload == nil {
		return false, false
	}
	raw, ok := payload[key]
	if !ok || raw == nil {
		return false, false
	}
	switch v := raw.(type) {
	case bool:
		return v, true
	case string:
		parsed, err := strconv.ParseBool(v)
		if err == nil {
			return parsed, true
		}
	}
	return false, false
}
