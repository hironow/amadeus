package domain

import "strconv"

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

const ImprovementSchemaVersion = "1"

const (
	MetadataFailureType              = "failure_type"
	MetadataSecondaryType            = "secondary_type"
	MetadataTargetAgent              = "target_agent"
	MetadataRecurrenceCount          = "recurrence_count"
	MetadataCorrectiveAction         = "corrective_action"
	MetadataCorrelationID            = "correlation_id"
	MetadataTraceID                  = "trace_id"
	MetadataOutcome                  = "outcome"
	MetadataImprovementSchemaVersion = "improvement_schema_version"
)

type CorrectionMetadata struct {
	SchemaVersion    string
	FailureType      FailureType
	SecondaryType    string
	TargetAgent      string
	RecurrenceCount  int
	CorrectiveAction string
	CorrelationID    string
	TraceID          string
	Outcome          ImprovementOutcome
}

type ImprovementEvent struct {
	SchemaVersion    string             `json:"schema_version" yaml:"schema_version"`
	FailureType      FailureType        `json:"failure_type" yaml:"failure_type"`
	SecondaryType    string             `json:"secondary_type,omitempty" yaml:"secondary_type,omitempty"`
	TargetAgent      string             `json:"target_agent,omitempty" yaml:"target_agent,omitempty"`
	RecurrenceCount  int                `json:"recurrence_count,omitempty" yaml:"recurrence_count,omitempty"`
	CorrectiveAction string             `json:"corrective_action,omitempty" yaml:"corrective_action,omitempty"`
	CorrelationID    string             `json:"correlation_id,omitempty" yaml:"correlation_id,omitempty"`
	TraceID          string             `json:"trace_id,omitempty" yaml:"trace_id,omitempty"`
	Outcome          ImprovementOutcome `json:"outcome,omitempty" yaml:"outcome,omitempty"`
}

func CorrectionMetadataFromMap(meta map[string]string) CorrectionMetadata {
	if len(meta) == 0 {
		return CorrectionMetadata{}
	}
	recurrence, _ := strconv.Atoi(meta[MetadataRecurrenceCount])
	return CorrectionMetadata{
		SchemaVersion:    meta[MetadataImprovementSchemaVersion],
		FailureType:      FailureType(meta[MetadataFailureType]),
		SecondaryType:    meta[MetadataSecondaryType],
		TargetAgent:      meta[MetadataTargetAgent],
		RecurrenceCount:  recurrence,
		CorrectiveAction: meta[MetadataCorrectiveAction],
		CorrelationID:    meta[MetadataCorrelationID],
		TraceID:          meta[MetadataTraceID],
		Outcome:          ImprovementOutcome(meta[MetadataOutcome]),
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
	if m.SecondaryType != "" {
		cp[MetadataSecondaryType] = m.SecondaryType
	}
	if m.TargetAgent != "" {
		cp[MetadataTargetAgent] = m.TargetAgent
	}
	if m.RecurrenceCount > 0 {
		cp[MetadataRecurrenceCount] = strconv.Itoa(m.RecurrenceCount)
	}
	if m.CorrectiveAction != "" {
		cp[MetadataCorrectiveAction] = m.CorrectiveAction
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
		SecondaryType:    m.SecondaryType,
		TargetAgent:      m.TargetAgent,
		RecurrenceCount:  m.RecurrenceCount,
		CorrectiveAction: m.CorrectiveAction,
		CorrelationID:    m.CorrelationID,
		TraceID:          m.TraceID,
		Outcome:          m.Outcome,
	}
}

func (m CorrectionMetadata) ForwardForRecheck() CorrectionMetadata {
	forwarded := m
	if forwarded.SchemaVersion == "" {
		forwarded.SchemaVersion = ImprovementSchemaVersion
	}
	forwarded.TargetAgent = ""
	if forwarded.Outcome == "" {
		forwarded.Outcome = ImprovementOutcomePending
	}
	return forwarded
}
