package domain

import (
	"strconv"
	"strings"
)

type FailureType string

const (
	FailureTypeNone              FailureType = "none"
	FailureTypeExecutionFailure  FailureType = "execution_failure"
	FailureTypeProviderFailure   FailureType = "provider_failure"
	FailureTypeScopeViolation    FailureType = "scope_violation"
	FailureTypeMissingAcceptance FailureType = "missing_acceptance_criteria"
	FailureTypeRoutingFailure    FailureType = "routing_failure"
	FailureTypeRecurrence        FailureType = "recurrence"
)

type ImprovementOutcome string

const (
	ImprovementOutcomePending     ImprovementOutcome = "pending"
	ImprovementOutcomeResolved    ImprovementOutcome = "resolved"
	ImprovementOutcomeEscalated   ImprovementOutcome = "escalated"
	ImprovementOutcomeFailedAgain ImprovementOutcome = "failed_again"
	ImprovementOutcomeIgnored     ImprovementOutcome = "ignored"
)

type RoutingMode string

const (
	RoutingModeRetry    RoutingMode = "retry"
	RoutingModeReroute  RoutingMode = "reroute"
	RoutingModeEscalate RoutingMode = "escalate"
)

const ImprovementSchemaVersion = "1"

const (
	MetadataFailureType              = "failure_type"
	MetadataSeverity                 = "severity"
	MetadataSecondaryType            = "secondary_type"
	MetadataTargetAgent              = "target_agent"
	MetadataRoutingMode              = "routing_mode"
	MetadataRoutingHistory           = "routing_history"
	MetadataOwnerHistory             = "owner_history"
	MetadataRecurrenceCount          = "recurrence_count"
	MetadataCorrectiveAction         = "corrective_action"
	MetadataRetryAllowed             = "retry_allowed"
	MetadataEscalationReason         = "escalation_reason"
	MetadataCorrelationID            = "correlation_id"
	MetadataTraceID                  = "trace_id"
	MetadataOutcome                  = "outcome"
	MetadataImprovementSchemaVersion = "improvement_schema_version"
)

type CorrectionMetadata struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go — domain metadata carrier; RoutingHistory/OwnerHistory are audit trail lists, not managed collections [permanent]
	SchemaVersion       string
	FailureType         FailureType
	Severity            Severity
	SecondaryType       string
	TargetAgent         string
	RoutingMode         RoutingMode
	RoutingHistory      []string
	OwnerHistory        []string
	RecurrenceCount     int
	CorrectiveAction    string
	RetryAllowed        *bool
	EscalationReason    string
	CorrelationID       string
	TraceID             string
	ProviderState       ProviderState
	ProviderReason      string
	ProviderRetryBudget int
	ProviderResumeAt    string
	ProviderResumeWhen  string
	Outcome             ImprovementOutcome
}

type ImprovementEvent struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go — JSON/YAML serialization struct for improvement event; RoutingHistory/OwnerHistory are event-sourced audit lists [permanent]
	SchemaVersion       string             `json:"schema_version" yaml:"schema_version"`
	FailureType         FailureType        `json:"failure_type" yaml:"failure_type"`
	Severity            Severity           `json:"severity,omitempty" yaml:"severity,omitempty"`
	SecondaryType       string             `json:"secondary_type,omitempty" yaml:"secondary_type,omitempty"`
	TargetAgent         string             `json:"target_agent,omitempty" yaml:"target_agent,omitempty"`
	RoutingMode         RoutingMode        `json:"routing_mode,omitempty" yaml:"routing_mode,omitempty"`
	RoutingHistory      []string           `json:"routing_history,omitempty" yaml:"routing_history,omitempty"`
	OwnerHistory        []string           `json:"owner_history,omitempty" yaml:"owner_history,omitempty"`
	RecurrenceCount     int                `json:"recurrence_count,omitempty" yaml:"recurrence_count,omitempty"`
	CorrectiveAction    string             `json:"corrective_action,omitempty" yaml:"corrective_action,omitempty"`
	RetryAllowed        *bool              `json:"retry_allowed,omitempty" yaml:"retry_allowed,omitempty"`
	EscalationReason    string             `json:"escalation_reason,omitempty" yaml:"escalation_reason,omitempty"`
	CorrelationID       string             `json:"correlation_id,omitempty" yaml:"correlation_id,omitempty"`
	TraceID             string             `json:"trace_id,omitempty" yaml:"trace_id,omitempty"`
	ProviderState       ProviderState      `json:"provider_state,omitempty" yaml:"provider_state,omitempty"`
	ProviderReason      string             `json:"provider_reason,omitempty" yaml:"provider_reason,omitempty"`
	ProviderRetryBudget int                `json:"provider_retry_budget,omitempty" yaml:"provider_retry_budget,omitempty"`
	ProviderResumeAt    string             `json:"provider_resume_at,omitempty" yaml:"provider_resume_at,omitempty"`
	ProviderResumeWhen  string             `json:"provider_resume_when,omitempty" yaml:"provider_resume_when,omitempty"`
	Outcome             ImprovementOutcome `json:"outcome,omitempty" yaml:"outcome,omitempty"`
}

func (m CorrectionMetadata) IsImprovement() bool {
	return m.SchemaVersion != "" ||
		m.FailureType != "" ||
		m.Severity != "" ||
		m.SecondaryType != "" ||
		m.TargetAgent != "" ||
		m.RoutingMode != "" ||
		len(m.RoutingHistory) > 0 ||
		len(m.OwnerHistory) > 0 ||
		m.RecurrenceCount > 0 ||
		m.CorrectiveAction != "" ||
		m.RetryAllowed != nil ||
		m.EscalationReason != "" ||
		m.CorrelationID != "" ||
		m.TraceID != "" ||
		m.Outcome != ""
}

func (m CorrectionMetadata) ConsumerSchemaVersion() string {
	if m.SchemaVersion != "" {
		return m.SchemaVersion
	}
	if m.IsImprovement() {
		return ImprovementSchemaVersion
	}
	return ""
}

func (m CorrectionMetadata) HasSupportedVocabulary() bool {
	return (m.Severity == "" || IsKnownSeverity(m.Severity)) &&
		(m.RoutingMode == "" || IsKnownRoutingMode(m.RoutingMode)) &&
		(m.ProviderState == "" || IsKnownProviderState(m.ProviderState)) &&
		(m.Outcome == "" || IsKnownImprovementOutcome(m.Outcome))
}

func NormalizeRoutingMode(mode RoutingMode) RoutingMode {
	switch RoutingMode(strings.ToLower(string(mode))) {
	case RoutingModeRetry:
		return RoutingModeRetry
	case RoutingModeReroute:
		return RoutingModeReroute
	case RoutingModeEscalate:
		return RoutingModeEscalate
	default:
		return mode
	}
}

func IsKnownRoutingMode(mode RoutingMode) bool {
	switch NormalizeRoutingMode(mode) {
	case RoutingModeRetry, RoutingModeReroute, RoutingModeEscalate:
		return true
	default:
		return false
	}
}

func NormalizeImprovementOutcome(outcome ImprovementOutcome) ImprovementOutcome {
	switch ImprovementOutcome(strings.ToLower(string(outcome))) {
	case ImprovementOutcomePending:
		return ImprovementOutcomePending
	case ImprovementOutcomeResolved:
		return ImprovementOutcomeResolved
	case ImprovementOutcomeEscalated:
		return ImprovementOutcomeEscalated
	case ImprovementOutcomeFailedAgain:
		return ImprovementOutcomeFailedAgain
	case ImprovementOutcomeIgnored:
		return ImprovementOutcomeIgnored
	default:
		return outcome
	}
}

func IsKnownImprovementOutcome(outcome ImprovementOutcome) bool {
	switch NormalizeImprovementOutcome(outcome) {
	case ImprovementOutcomePending, ImprovementOutcomeResolved, ImprovementOutcomeEscalated, ImprovementOutcomeFailedAgain, ImprovementOutcomeIgnored:
		return true
	default:
		return false
	}
}

func IsKnownSeverity(severity Severity) bool {
	switch NormalizeSeverity(severity) {
	case SeverityLow, SeverityMedium, SeverityHigh:
		return true
	default:
		return false
	}
}

func CorrectionMetadataFromMap(meta map[string]string) CorrectionMetadata {
	if len(meta) == 0 {
		return CorrectionMetadata{}
	}
	recurrence := 0
	if raw := meta[MetadataRecurrenceCount]; raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			recurrence = v
		}
	}
	providerRetryBudget := 0
	if raw := meta[MetadataProviderRetryBudget]; raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			providerRetryBudget = v
		}
	}
	var retryAllowed *bool
	if raw, ok := meta[MetadataRetryAllowed]; ok && raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err == nil {
			retryAllowed = BoolPtr(parsed)
		}
	}
	return CorrectionMetadata{
		SchemaVersion:       meta[MetadataImprovementSchemaVersion],
		FailureType:         FailureType(meta[MetadataFailureType]),
		Severity:            NormalizeSeverity(Severity(meta[MetadataSeverity])),
		SecondaryType:       meta[MetadataSecondaryType],
		TargetAgent:         meta[MetadataTargetAgent],
		RoutingMode:         NormalizeRoutingMode(RoutingMode(meta[MetadataRoutingMode])),
		RoutingHistory:      parseImprovementHistory(meta[MetadataRoutingHistory]),
		OwnerHistory:        parseImprovementHistory(meta[MetadataOwnerHistory]),
		RecurrenceCount:     recurrence,
		CorrectiveAction:    meta[MetadataCorrectiveAction],
		RetryAllowed:        retryAllowed,
		EscalationReason:    meta[MetadataEscalationReason],
		CorrelationID:       meta[MetadataCorrelationID],
		TraceID:             meta[MetadataTraceID],
		ProviderState:       NormalizeProviderState(ProviderState(meta[MetadataProviderState])),
		ProviderReason:      meta[MetadataProviderReason],
		ProviderRetryBudget: providerRetryBudget,
		ProviderResumeAt:    meta[MetadataProviderResumeAt],
		ProviderResumeWhen:  meta[MetadataProviderResumeWhen],
		Outcome:             NormalizeImprovementOutcome(ImprovementOutcome(meta[MetadataOutcome])),
	}
}

func (m CorrectionMetadata) Apply(meta map[string]string) map[string]string {
	cp := make(map[string]string, len(meta)+13)
	for k, v := range meta {
		cp[k] = v
	}
	schemaVersion := m.SchemaVersion
	if schemaVersion == "" {
		schemaVersion = ImprovementSchemaVersion
	}
	cp[MetadataImprovementSchemaVersion] = schemaVersion
	if m.FailureType != "" {
		cp[MetadataFailureType] = string(m.FailureType)
	}
	if m.Severity != "" {
		cp[MetadataSeverity] = string(NormalizeSeverity(m.Severity))
	}
	if m.SecondaryType != "" {
		cp[MetadataSecondaryType] = m.SecondaryType
	}
	if m.TargetAgent != "" {
		cp[MetadataTargetAgent] = m.TargetAgent
	}
	if m.RoutingMode != "" {
		cp[MetadataRoutingMode] = string(NormalizeRoutingMode(m.RoutingMode))
	}
	if len(m.RoutingHistory) > 0 {
		cp[MetadataRoutingHistory] = formatImprovementHistory(m.RoutingHistory)
	}
	if len(m.OwnerHistory) > 0 {
		cp[MetadataOwnerHistory] = formatImprovementHistory(m.OwnerHistory)
	}
	if m.RecurrenceCount > 0 {
		cp[MetadataRecurrenceCount] = strconv.Itoa(m.RecurrenceCount)
	}
	if m.CorrectiveAction != "" {
		cp[MetadataCorrectiveAction] = m.CorrectiveAction
	}
	if m.RetryAllowed != nil {
		cp[MetadataRetryAllowed] = strconv.FormatBool(*m.RetryAllowed)
	}
	if m.EscalationReason != "" {
		cp[MetadataEscalationReason] = m.EscalationReason
	}
	if m.CorrelationID != "" {
		cp[MetadataCorrelationID] = m.CorrelationID
	}
	if m.TraceID != "" {
		cp[MetadataTraceID] = m.TraceID
	}
	if m.ProviderState != "" {
		cp[MetadataProviderState] = string(NormalizeProviderState(m.ProviderState))
	}
	if m.ProviderReason != "" {
		cp[MetadataProviderReason] = m.ProviderReason
	}
	if m.ProviderState != "" || m.ProviderReason != "" || m.ProviderRetryBudget != 0 || m.ProviderResumeAt != "" || m.ProviderResumeWhen != "" {
		cp[MetadataProviderRetryBudget] = strconv.Itoa(m.ProviderRetryBudget)
	}
	if m.ProviderResumeAt != "" {
		cp[MetadataProviderResumeAt] = m.ProviderResumeAt
	}
	if m.ProviderResumeWhen != "" {
		cp[MetadataProviderResumeWhen] = m.ProviderResumeWhen
	}
	if m.Outcome != "" {
		cp[MetadataOutcome] = string(m.Outcome)
	}
	return cp
}

func (m CorrectionMetadata) ImprovementEvent() ImprovementEvent {
	schemaVersion := m.SchemaVersion
	if schemaVersion == "" {
		schemaVersion = ImprovementSchemaVersion
	}
	return ImprovementEvent{
		SchemaVersion:       schemaVersion,
		FailureType:         m.FailureType,
		Severity:            NormalizeSeverity(m.Severity),
		SecondaryType:       m.SecondaryType,
		TargetAgent:         m.TargetAgent,
		RoutingMode:         NormalizeRoutingMode(m.RoutingMode),
		RoutingHistory:      append([]string(nil), m.RoutingHistory...),
		OwnerHistory:        append([]string(nil), m.OwnerHistory...),
		RecurrenceCount:     m.RecurrenceCount,
		CorrectiveAction:    m.CorrectiveAction,
		RetryAllowed:        m.RetryAllowed,
		EscalationReason:    m.EscalationReason,
		CorrelationID:       m.CorrelationID,
		TraceID:             m.TraceID,
		ProviderState:       NormalizeProviderState(m.ProviderState),
		ProviderReason:      m.ProviderReason,
		ProviderRetryBudget: m.ProviderRetryBudget,
		ProviderResumeAt:    m.ProviderResumeAt,
		ProviderResumeWhen:  m.ProviderResumeWhen,
		Outcome:             m.Outcome,
	}
}

func (m CorrectionMetadata) ForwardForRecheck() CorrectionMetadata {
	forwarded := m
	if !forwarded.IsImprovement() {
		return forwarded
	}
	if forwarded.SchemaVersion == "" {
		forwarded.SchemaVersion = ImprovementSchemaVersion
	}
	forwarded.TargetAgent = ""
	forwarded.RoutingMode = ""
	if forwarded.Outcome == "" {
		forwarded.Outcome = ImprovementOutcomePending
	}
	return forwarded
}

func ParseImprovementHistory(raw string) []string {
	return parseImprovementHistory(raw)
}

func FormatImprovementHistory(values []string) string {
	return formatImprovementHistory(values)
}

func AppendImprovementHistory(history []string, value string) []string {
	return appendImprovementHistory(history, value)
}

func parseImprovementHistory(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	// Canonical delimiter is ">". Fall back to "," for legacy data
	// that used comma-separated values (Postel's Law: liberal parse).
	delimiter := ">"
	if !strings.Contains(raw, ">") && strings.Contains(raw, ",") {
		delimiter = ","
	}
	parts := strings.Split(raw, delimiter)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = appendImprovementHistory(out, part)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func formatImprovementHistory(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(normalizeImprovementHistory(values), ">")
}

func appendImprovementHistory(history []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return append([]string(nil), history...)
	}
	next := append([]string(nil), history...)
	if len(next) > 0 && next[len(next)-1] == value {
		return next
	}
	return append(next, value)
}

func normalizeImprovementHistory(values []string) []string {
	var out []string
	for _, value := range values {
		out = appendImprovementHistory(out, value)
	}
	return out
}

func BoolPtr(v bool) *bool {
	return &v
}
