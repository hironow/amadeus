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
	MetadataRecurrenceCount          = "recurrence_count"
	MetadataCorrectiveAction         = "corrective_action"
	MetadataRetryAllowed             = "retry_allowed"
	MetadataEscalationReason         = "escalation_reason"
	MetadataCorrelationID            = "correlation_id"
	MetadataTraceID                  = "trace_id"
	MetadataOutcome                  = "outcome"
	MetadataImprovementSchemaVersion = "improvement_schema_version"
)

type CorrectionMetadata struct {
	SchemaVersion    string
	FailureType      FailureType
	Severity         Severity
	SecondaryType    string
	TargetAgent      string
	RoutingMode      RoutingMode
	RecurrenceCount  int
	CorrectiveAction string
	RetryAllowed     *bool
	EscalationReason string
	CorrelationID    string
	TraceID          string
	Outcome          ImprovementOutcome
}

type ImprovementEvent struct {
	SchemaVersion    string             `json:"schema_version" yaml:"schema_version"`
	FailureType      FailureType        `json:"failure_type" yaml:"failure_type"`
	Severity         Severity           `json:"severity,omitempty" yaml:"severity,omitempty"`
	SecondaryType    string             `json:"secondary_type,omitempty" yaml:"secondary_type,omitempty"`
	TargetAgent      string             `json:"target_agent,omitempty" yaml:"target_agent,omitempty"`
	RoutingMode      RoutingMode        `json:"routing_mode,omitempty" yaml:"routing_mode,omitempty"`
	RecurrenceCount  int                `json:"recurrence_count,omitempty" yaml:"recurrence_count,omitempty"`
	CorrectiveAction string             `json:"corrective_action,omitempty" yaml:"corrective_action,omitempty"`
	RetryAllowed     *bool              `json:"retry_allowed,omitempty" yaml:"retry_allowed,omitempty"`
	EscalationReason string             `json:"escalation_reason,omitempty" yaml:"escalation_reason,omitempty"`
	CorrelationID    string             `json:"correlation_id,omitempty" yaml:"correlation_id,omitempty"`
	TraceID          string             `json:"trace_id,omitempty" yaml:"trace_id,omitempty"`
	Outcome          ImprovementOutcome `json:"outcome,omitempty" yaml:"outcome,omitempty"`
}

func (m CorrectionMetadata) IsImprovement() bool {
	return m.SchemaVersion != "" ||
		m.FailureType != "" ||
		m.Severity != "" ||
		m.SecondaryType != "" ||
		m.TargetAgent != "" ||
		m.RoutingMode != "" ||
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
	recurrence, _ := strconv.Atoi(meta[MetadataRecurrenceCount])
	var retryAllowed *bool
	if raw, ok := meta[MetadataRetryAllowed]; ok && raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err == nil {
			retryAllowed = BoolPtr(parsed)
		}
	}
	return CorrectionMetadata{
		SchemaVersion:    meta[MetadataImprovementSchemaVersion],
		FailureType:      FailureType(meta[MetadataFailureType]),
		Severity:         NormalizeSeverity(Severity(meta[MetadataSeverity])),
		SecondaryType:    meta[MetadataSecondaryType],
		TargetAgent:      meta[MetadataTargetAgent],
		RoutingMode:      NormalizeRoutingMode(RoutingMode(meta[MetadataRoutingMode])),
		RecurrenceCount:  recurrence,
		CorrectiveAction: meta[MetadataCorrectiveAction],
		RetryAllowed:     retryAllowed,
		EscalationReason: meta[MetadataEscalationReason],
		CorrelationID:    meta[MetadataCorrelationID],
		TraceID:          meta[MetadataTraceID],
		Outcome:          NormalizeImprovementOutcome(ImprovementOutcome(meta[MetadataOutcome])),
	}
}

func (m CorrectionMetadata) Apply(meta map[string]string) map[string]string {
	cp := make(map[string]string, len(meta)+8)
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
		SchemaVersion:    schemaVersion,
		FailureType:      m.FailureType,
		Severity:         NormalizeSeverity(m.Severity),
		SecondaryType:    m.SecondaryType,
		TargetAgent:      m.TargetAgent,
		RoutingMode:      NormalizeRoutingMode(m.RoutingMode),
		RecurrenceCount:  m.RecurrenceCount,
		CorrectiveAction: m.CorrectiveAction,
		RetryAllowed:     m.RetryAllowed,
		EscalationReason: m.EscalationReason,
		CorrelationID:    m.CorrelationID,
		TraceID:          m.TraceID,
		Outcome:          m.Outcome,
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

func BoolPtr(v bool) *bool {
	return &v
}
