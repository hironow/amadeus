package domain_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestCorrectionMetadataApplyRoundTrip(t *testing.T) {
	meta := domain.CorrectionMetadata{
		FailureType:      domain.FailureTypeExecutionFailure,
		SecondaryType:    "implementation",
		TargetAgent:      "paintress",
		RecurrenceCount:  2,
		CorrectiveAction: "retry",
		CorrelationID:    "corr-1",
		TraceID:          "trace-1",
		Outcome:          domain.ImprovementOutcomePending,
	}

	applied := meta.Apply(map[string]string{"existing": "ok"})
	got := domain.CorrectionMetadataFromMap(applied)

	if got.FailureType != meta.FailureType {
		t.Fatalf("FailureType = %q, want %q", got.FailureType, meta.FailureType)
	}
	if got.TargetAgent != "paintress" {
		t.Fatalf("TargetAgent = %q, want paintress", got.TargetAgent)
	}
	if got.RecurrenceCount != 2 {
		t.Fatalf("RecurrenceCount = %d, want 2", got.RecurrenceCount)
	}
	if applied["existing"] != "ok" {
		t.Fatal("existing metadata was lost")
	}
	if applied[domain.MetadataImprovementSchemaVersion] != domain.ImprovementSchemaVersion {
		t.Fatalf("schema version = %q, want %q", applied[domain.MetadataImprovementSchemaVersion], domain.ImprovementSchemaVersion)
	}
}
