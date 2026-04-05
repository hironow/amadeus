package domain_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestCorrectionMetadataApplyRoundTrip(t *testing.T) {
	meta := domain.CorrectionMetadata{
		FailureType:      domain.FailureTypeExecutionFailure,
		Severity:         domain.SeverityMedium,
		SecondaryType:    "implementation",
		TargetAgent:      "paintress",
		RecurrenceCount:  2,
		CorrectiveAction: "retry",
		RetryAllowed:     domain.BoolPtr(true),
		EscalationReason: "recurrence-threshold",
		CorrelationID:    "corr-1",
		TraceID:          "trace-1",
		Outcome:          domain.ImprovementOutcomePending,
	}

	applied := meta.Apply(map[string]string{"existing": "ok"})
	got := domain.CorrectionMetadataFromMap(applied)

	if got.FailureType != meta.FailureType {
		t.Fatalf("FailureType = %q, want %q", got.FailureType, meta.FailureType)
	}
	if got.Severity != meta.Severity {
		t.Fatalf("Severity = %q, want %q", got.Severity, meta.Severity)
	}
	if got.TargetAgent != "paintress" {
		t.Fatalf("TargetAgent = %q, want paintress", got.TargetAgent)
	}
	if got.RecurrenceCount != 2 {
		t.Fatalf("RecurrenceCount = %d, want 2", got.RecurrenceCount)
	}
	if got.RetryAllowed == nil || !*got.RetryAllowed {
		t.Fatal("RetryAllowed = nil/false, want true")
	}
	if got.EscalationReason != "recurrence-threshold" {
		t.Fatalf("EscalationReason = %q, want recurrence-threshold", got.EscalationReason)
	}
	if applied["existing"] != "ok" {
		t.Fatal("existing metadata was lost")
	}
	if applied[domain.MetadataImprovementSchemaVersion] != domain.ImprovementSchemaVersion {
		t.Fatalf("schema version = %q, want %q", applied[domain.MetadataImprovementSchemaVersion], domain.ImprovementSchemaVersion)
	}
	if applied[domain.MetadataSeverity] != string(domain.SeverityMedium) {
		t.Fatalf("severity = %q, want %q", applied[domain.MetadataSeverity], domain.SeverityMedium)
	}
}

func TestCorrectionMetadataForwardForRecheck(t *testing.T) {
	meta := domain.CorrectionMetadata{
		FailureType:      domain.FailureTypeExecutionFailure,
		TargetAgent:      "paintress",
		CorrelationID:    "corr-1",
		CorrectiveAction: "retry",
	}

	got := meta.ForwardForRecheck()

	if got.TargetAgent != "" {
		t.Fatalf("TargetAgent = %q, want empty", got.TargetAgent)
	}
	if got.Outcome != domain.ImprovementOutcomePending {
		t.Fatalf("Outcome = %q, want %q", got.Outcome, domain.ImprovementOutcomePending)
	}
	if got.SchemaVersion != domain.ImprovementSchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", got.SchemaVersion, domain.ImprovementSchemaVersion)
	}
	if got.CorrelationID != meta.CorrelationID {
		t.Fatalf("CorrelationID = %q, want %q", got.CorrelationID, meta.CorrelationID)
	}
	if got.RetryAllowed != nil {
		t.Fatalf("RetryAllowed = %v, want nil", *got.RetryAllowed)
	}
}

func TestCorrectionMetadataFromMap_LegacyV1WithoutSchemaVersion(t *testing.T) {
	got := domain.CorrectionMetadataFromMap(map[string]string{
		domain.MetadataFailureType: "execution_failure",
		domain.MetadataSeverity:    "HIGH",
		domain.MetadataOutcome:     "FAILED_AGAIN",
	})

	if !got.IsImprovement() {
		t.Fatal("IsImprovement = false, want true")
	}
	if got.ConsumerSchemaVersion() != domain.ImprovementSchemaVersion {
		t.Fatalf("ConsumerSchemaVersion = %q, want %q", got.ConsumerSchemaVersion(), domain.ImprovementSchemaVersion)
	}
	if got.Severity != domain.SeverityHigh {
		t.Fatalf("Severity = %q, want %q", got.Severity, domain.SeverityHigh)
	}
	if got.Outcome != domain.ImprovementOutcomeFailedAgain {
		t.Fatalf("Outcome = %q, want %q", got.Outcome, domain.ImprovementOutcomeFailedAgain)
	}
	if !got.HasSupportedVocabulary() {
		t.Fatal("HasSupportedVocabulary = false, want true")
	}
}

func TestCorrectionMetadataHasSupportedVocabulary_RejectsUnknownOutcome(t *testing.T) {
	meta := domain.CorrectionMetadata{
		FailureType: domain.FailureTypeExecutionFailure,
		Severity:    domain.SeverityMedium,
		Outcome:     domain.ImprovementOutcome("not-real"),
	}

	if meta.HasSupportedVocabulary() {
		t.Fatal("HasSupportedVocabulary = true, want false")
	}
}
