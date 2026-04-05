package policy_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness/policy"
)

func TestDetermineCorrectionDecision_ReroutesImplementationFeedbackToSightjackForDesignFailure(t *testing.T) {
	got := policy.DetermineCorrectionDecision(
		domain.KindImplFeedback,
		domain.SeverityMedium,
		domain.ActionRetry,
		domain.FailureTypeScopeViolation,
		1,
		domain.CorrectionMetadata{},
	)

	if got.TargetAgent != "sightjack" {
		t.Fatalf("TargetAgent = %q, want sightjack", got.TargetAgent)
	}
	if got.RoutingMode != domain.RoutingModeReroute {
		t.Fatalf("RoutingMode = %q, want %q", got.RoutingMode, domain.RoutingModeReroute)
	}
	if got.RetryAllowed == nil || !*got.RetryAllowed {
		t.Fatal("RetryAllowed = nil/false, want true")
	}
}

func TestDetermineCorrectionDecision_EscalatesWhenRetryDisabled(t *testing.T) {
	got := policy.DetermineCorrectionDecision(
		domain.KindImplFeedback,
		domain.SeverityMedium,
		domain.ActionRetry,
		domain.FailureTypeExecutionFailure,
		1,
		domain.CorrectionMetadata{RetryAllowed: domain.BoolPtr(false)},
	)

	if got.Action != domain.ActionEscalate {
		t.Fatalf("Action = %q, want %q", got.Action, domain.ActionEscalate)
	}
	if got.RoutingMode != domain.RoutingModeEscalate {
		t.Fatalf("RoutingMode = %q, want %q", got.RoutingMode, domain.RoutingModeEscalate)
	}
	if got.EscalationReason != "retry-disabled" {
		t.Fatalf("EscalationReason = %q, want retry-disabled", got.EscalationReason)
	}
}

func TestDetermineCorrectionDecision_EscalatesAfterRecurrenceThreshold(t *testing.T) {
	got := policy.DetermineCorrectionDecision(
		domain.KindImplFeedback,
		domain.SeverityMedium,
		domain.ActionRetry,
		domain.FailureTypeExecutionFailure,
		2,
		domain.CorrectionMetadata{},
	)

	if got.Action != domain.ActionEscalate {
		t.Fatalf("Action = %q, want %q", got.Action, domain.ActionEscalate)
	}
	if got.RoutingMode != domain.RoutingModeEscalate {
		t.Fatalf("RoutingMode = %q, want %q", got.RoutingMode, domain.RoutingModeEscalate)
	}
	if got.TargetAgent != "paintress" {
		t.Fatalf("TargetAgent = %q, want paintress", got.TargetAgent)
	}
	if got.EscalationReason != "recurrence-threshold" {
		t.Fatalf("EscalationReason = %q, want recurrence-threshold", got.EscalationReason)
	}
}
