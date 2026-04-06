package domain_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestDefaultRoutingPolicy(t *testing.T) {
	// when
	policy := domain.DefaultRoutingPolicy()

	// then
	if policy.RecurrenceThreshold != 2 {
		t.Errorf("RecurrenceThreshold = %d, want 2", policy.RecurrenceThreshold)
	}
	if policy.SeverityActionMap[domain.SeverityHigh] != "escalate" {
		t.Errorf("SeverityActionMap[High] = %q, want escalate", policy.SeverityActionMap[domain.SeverityHigh])
	}
	if policy.TargetAgentMap[domain.FailureTypeScopeViolation] != "sightjack" {
		t.Errorf("TargetAgentMap[ScopeViolation] = %q, want sightjack", policy.TargetAgentMap[domain.FailureTypeScopeViolation])
	}
	if policy.TargetAgentMap[domain.FailureTypeExecutionFailure] != "paintress" {
		t.Errorf("TargetAgentMap[ExecutionFailure] = %q, want paintress", policy.TargetAgentMap[domain.FailureTypeExecutionFailure])
	}
}

func TestRoutingPolicy_LookupSeverityAction(t *testing.T) {
	policy := domain.DefaultRoutingPolicy()

	tests := []struct {
		severity domain.Severity
		want     string
	}{
		{domain.SeverityHigh, "escalate"},
		{domain.SeverityMedium, "retry"},
		{domain.SeverityLow, "retry"},
		{domain.Severity("unknown"), "retry"}, // fallback
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			got := policy.LookupSeverityAction(tt.severity)
			if got != tt.want {
				t.Errorf("LookupSeverityAction(%s) = %q, want %q", tt.severity, got, tt.want)
			}
		})
	}
}

func TestRoutingPolicy_LookupTargetAgent(t *testing.T) {
	policy := domain.DefaultRoutingPolicy()

	tests := []struct {
		ft   domain.FailureType
		want string
	}{
		{domain.FailureTypeScopeViolation, "sightjack"},
		{domain.FailureTypeMissingAcceptance, "sightjack"},
		{domain.FailureTypeExecutionFailure, "paintress"},
		{domain.FailureTypeProviderFailure, "paintress"},
		{domain.FailureTypeNone, ""}, // no override
	}

	for _, tt := range tests {
		t.Run(string(tt.ft), func(t *testing.T) {
			got := policy.LookupTargetAgent(tt.ft)
			if got != tt.want {
				t.Errorf("LookupTargetAgent(%s) = %q, want %q", tt.ft, got, tt.want)
			}
		})
	}
}
