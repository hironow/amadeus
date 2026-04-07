package session

import (
	"context"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"go.opentelemetry.io/otel/trace"
)

// white-box-reason: session internals: tests unexported corrective metadata decision helper

func TestDMailCorrectionMetadata_AllowsRetryForFirstMediumPass(t *testing.T) {
	meta := dmailCorrectionMetadata(
		domain.ClaudeDMailCandidate{Category: "implementation", Action: "retry"},
		domain.KindImplFeedback,
		"feedback-1",
		domain.SeverityMedium,
		nil,
		1,
		domain.CorrectionMetadata{},
		domain.DefaultRoutingPolicy(),
		trace.SpanFromContext(context.Background()),
	)

	if meta.RetryAllowed == nil || !*meta.RetryAllowed {
		t.Fatal("RetryAllowed = nil/false, want true")
	}
	if meta.CorrectiveAction != string(domain.ActionRetry) {
		t.Fatalf("CorrectiveAction = %q, want %q", meta.CorrectiveAction, domain.ActionRetry)
	}
	if meta.SchemaVersion != domain.ImprovementSchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", meta.SchemaVersion, domain.ImprovementSchemaVersion)
	}
	if meta.Severity != domain.SeverityMedium {
		t.Fatalf("Severity = %q, want %q", meta.Severity, domain.SeverityMedium)
	}
	if meta.TargetAgent != "paintress" {
		t.Fatalf("TargetAgent = %q, want paintress", meta.TargetAgent)
	}
	if meta.RoutingMode != domain.RoutingModeRetry {
		t.Fatalf("RoutingMode = %q, want %q", meta.RoutingMode, domain.RoutingModeRetry)
	}
	if meta.EscalationReason != "" {
		t.Fatalf("EscalationReason = %q, want empty", meta.EscalationReason)
	}
	if meta.Outcome != domain.ImprovementOutcomePending {
		t.Fatalf("Outcome = %q, want %q", meta.Outcome, domain.ImprovementOutcomePending)
	}
	if got := domain.FormatImprovementHistory(meta.RoutingHistory); got != "retry" {
		t.Fatalf("RoutingHistory = %q, want retry", got)
	}
	if got := domain.FormatImprovementHistory(meta.OwnerHistory); got != "paintress" {
		t.Fatalf("OwnerHistory = %q, want paintress", got)
	}
}

func TestDMailCorrectionMetadata_EscalatesHighSeverity(t *testing.T) {
	meta := dmailCorrectionMetadata(
		domain.ClaudeDMailCandidate{Category: "implementation"},
		domain.KindImplFeedback,
		"feedback-2",
		domain.SeverityHigh,
		nil,
		1,
		domain.CorrectionMetadata{},
		domain.DefaultRoutingPolicy(),
		trace.SpanFromContext(context.Background()),
	)

	if meta.RetryAllowed == nil || *meta.RetryAllowed {
		t.Fatal("RetryAllowed = nil/true, want false")
	}
	if meta.CorrectiveAction != string(domain.ActionEscalate) {
		t.Fatalf("CorrectiveAction = %q, want %q", meta.CorrectiveAction, domain.ActionEscalate)
	}
	if meta.Severity != domain.SeverityHigh {
		t.Fatalf("Severity = %q, want %q", meta.Severity, domain.SeverityHigh)
	}
	if meta.TargetAgent != "paintress" {
		t.Fatalf("TargetAgent = %q, want paintress", meta.TargetAgent)
	}
	if meta.RoutingMode != domain.RoutingModeEscalate {
		t.Fatalf("RoutingMode = %q, want %q", meta.RoutingMode, domain.RoutingModeEscalate)
	}
	if meta.EscalationReason != "high-severity" {
		t.Fatalf("EscalationReason = %q, want high-severity", meta.EscalationReason)
	}
	if meta.Outcome != domain.ImprovementOutcomeEscalated {
		t.Fatalf("Outcome = %q, want %q", meta.Outcome, domain.ImprovementOutcomeEscalated)
	}
}

func TestDMailCorrectionMetadata_EscalatesAfterRecurrenceThreshold(t *testing.T) {
	meta := dmailCorrectionMetadata(
		domain.ClaudeDMailCandidate{Category: "implementation", Action: "retry"},
		domain.KindImplFeedback,
		"feedback-3",
		domain.SeverityMedium,
		nil,
		1,
		domain.CorrectionMetadata{
			SchemaVersion:   domain.ImprovementSchemaVersion,
			RoutingHistory:  []string{"retry"},
			OwnerHistory:    []string{"paintress"},
			RecurrenceCount: 1,
			RetryAllowed:    domain.BoolPtr(true),
		},
		domain.DefaultRoutingPolicy(),
		trace.SpanFromContext(context.Background()),
	)

	if meta.RecurrenceCount != 2 {
		t.Fatalf("RecurrenceCount = %d, want 2", meta.RecurrenceCount)
	}
	if meta.RetryAllowed == nil || *meta.RetryAllowed {
		t.Fatal("RetryAllowed = nil/true, want false")
	}
	if meta.CorrectiveAction != string(domain.ActionEscalate) {
		t.Fatalf("CorrectiveAction = %q, want %q", meta.CorrectiveAction, domain.ActionEscalate)
	}
	if meta.Severity != domain.SeverityHigh {
		t.Fatalf("Severity = %q, want %q", meta.Severity, domain.SeverityHigh)
	}
	if meta.TargetAgent != "paintress" {
		t.Fatalf("TargetAgent = %q, want paintress", meta.TargetAgent)
	}
	if meta.RoutingMode != domain.RoutingModeEscalate {
		t.Fatalf("RoutingMode = %q, want %q", meta.RoutingMode, domain.RoutingModeEscalate)
	}
	if meta.EscalationReason != "recurrence-threshold" {
		t.Fatalf("EscalationReason = %q, want recurrence-threshold", meta.EscalationReason)
	}
	if meta.Outcome != domain.ImprovementOutcomeEscalated {
		t.Fatalf("Outcome = %q, want %q", meta.Outcome, domain.ImprovementOutcomeEscalated)
	}
	if got := domain.FormatImprovementHistory(meta.RoutingHistory); got != "retry>escalate" {
		t.Fatalf("RoutingHistory = %q, want retry>escalate", got)
	}
	if got := domain.FormatImprovementHistory(meta.OwnerHistory); got != "paintress" {
		t.Fatalf("OwnerHistory = %q, want paintress", got)
	}
}

func TestDMailCorrectionMetadata_PreservesLegacyTriggerSchemaAsV1(t *testing.T) {
	meta := dmailCorrectionMetadata(
		domain.ClaudeDMailCandidate{Category: "implementation", Action: "retry"},
		domain.KindImplFeedback,
		"feedback-4",
		domain.SeverityMedium,
		nil,
		1,
		domain.CorrectionMetadata{
			FailureType:     domain.FailureTypeExecutionFailure,
			Severity:        domain.SeverityHigh,
			CorrelationID:   "corr-legacy",
			RecurrenceCount: 1,
			RetryAllowed:    domain.BoolPtr(true),
		},
		domain.DefaultRoutingPolicy(),
		trace.SpanFromContext(context.Background()),
	)

	if meta.SchemaVersion != domain.ImprovementSchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", meta.SchemaVersion, domain.ImprovementSchemaVersion)
	}
	if meta.CorrelationID != "corr-legacy" {
		t.Fatalf("CorrelationID = %q, want corr-legacy", meta.CorrelationID)
	}
	if meta.Severity != domain.SeverityHigh {
		t.Fatalf("Severity = %q, want %q", meta.Severity, domain.SeverityHigh)
	}
}

func TestDMailCorrectionMetadata_ReroutesImplementationFeedbackToSightjackForDesignFailure(t *testing.T) {
	meta := dmailCorrectionMetadata(
		domain.ClaudeDMailCandidate{Category: "design", Action: "retry"},
		domain.KindImplFeedback,
		"feedback-5",
		domain.SeverityMedium,
		nil,
		1,
		domain.CorrectionMetadata{},
		domain.DefaultRoutingPolicy(),
		trace.SpanFromContext(context.Background()),
	)

	if meta.TargetAgent != "sightjack" {
		t.Fatalf("TargetAgent = %q, want sightjack", meta.TargetAgent)
	}
	if meta.RoutingMode != domain.RoutingModeReroute {
		t.Fatalf("RoutingMode = %q, want %q", meta.RoutingMode, domain.RoutingModeReroute)
	}
	if meta.CorrectiveAction != string(domain.ActionRetry) {
		t.Fatalf("CorrectiveAction = %q, want %q", meta.CorrectiveAction, domain.ActionRetry)
	}
	if meta.RetryAllowed == nil || !*meta.RetryAllowed {
		t.Fatal("RetryAllowed = nil/false, want true")
	}
}

func TestDMailCorrectionMetadata_EscalatedDesignFailureKeepsSightjackAsHandoffOwner(t *testing.T) {
	meta := dmailCorrectionMetadata(
		domain.ClaudeDMailCandidate{Category: "design"},
		domain.KindImplFeedback,
		"feedback-6",
		domain.SeverityHigh,
		nil,
		1,
		domain.CorrectionMetadata{},
		domain.DefaultRoutingPolicy(),
		trace.SpanFromContext(context.Background()),
	)

	if meta.TargetAgent != "sightjack" {
		t.Fatalf("TargetAgent = %q, want sightjack", meta.TargetAgent)
	}
	if meta.RoutingMode != domain.RoutingModeEscalate {
		t.Fatalf("RoutingMode = %q, want %q", meta.RoutingMode, domain.RoutingModeEscalate)
	}
	if meta.Severity != domain.SeverityHigh {
		t.Fatalf("Severity = %q, want %q", meta.Severity, domain.SeverityHigh)
	}
}
