package policy_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness/policy"
)

// --- Existing tests (updated with providerSnapshot parameter) ---

func TestDetermineCorrectionDecision_ReroutesImplementationFeedbackToSightjackForDesignFailure(t *testing.T) {
	got := policy.DetermineCorrectionDecision(
		domain.KindImplFeedback,
		domain.SeverityMedium,
		domain.ActionRetry,
		domain.FailureTypeScopeViolation,
		1,
		domain.CorrectionMetadata{},
		domain.ActiveProviderState(), domain.DefaultRoutingPolicy(),
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
		domain.ActiveProviderState(), domain.DefaultRoutingPolicy(),
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
		domain.ActiveProviderState(), domain.DefaultRoutingPolicy(),
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

// --- C-1: Provider state gate tests ---

func TestDetermineCorrectionDecision_ProviderStateGate(t *testing.T) {
	tests := []struct {
		name             string
		snapshot         domain.ProviderStateSnapshot
		wantAction       domain.DMailAction
		wantMode         domain.RoutingMode
		wantReason       string
		wantRetryAllowed bool
	}{
		{
			name: "paused with any budget escalates",
			snapshot: domain.ProviderStateSnapshot{
				State:       domain.ProviderStatePaused,
				RetryBudget: 3,
			},
			wantAction:       domain.ActionEscalate,
			wantMode:         domain.RoutingModeEscalate,
			wantReason:       "provider-paused",
			wantRetryAllowed: false,
		},
		{
			name: "degraded with zero budget escalates",
			snapshot: domain.ProviderStateSnapshot{
				State:       domain.ProviderStateDegraded,
				RetryBudget: 0,
			},
			wantAction:       domain.ActionEscalate,
			wantMode:         domain.RoutingModeEscalate,
			wantReason:       "provider-degraded-exhausted",
			wantRetryAllowed: false,
		},
		{
			name: "waiting with zero budget escalates",
			snapshot: domain.ProviderStateSnapshot{
				State:       domain.ProviderStateWaiting,
				RetryBudget: 0,
			},
			wantAction:       domain.ActionEscalate,
			wantMode:         domain.RoutingModeEscalate,
			wantReason:       "provider-waiting-exhausted",
			wantRetryAllowed: false,
		},
		{
			name: "degraded with positive budget delegates to existing logic",
			snapshot: domain.ProviderStateSnapshot{
				State:       domain.ProviderStateDegraded,
				RetryBudget: 1,
			},
			wantAction:       domain.ActionRetry,
			wantMode:         domain.RoutingModeRetry,
			wantReason:       "",
			wantRetryAllowed: true,
		},
		{
			name: "waiting with positive budget delegates to existing logic",
			snapshot: domain.ProviderStateSnapshot{
				State:       domain.ProviderStateWaiting,
				RetryBudget: 1,
			},
			wantAction:       domain.ActionRetry,
			wantMode:         domain.RoutingModeRetry,
			wantReason:       "",
			wantRetryAllowed: true,
		},
		{
			name:             "active delegates to existing logic",
			snapshot:         domain.ActiveProviderState(),
			wantAction:       domain.ActionRetry,
			wantMode:         domain.RoutingModeRetry,
			wantReason:       "",
			wantRetryAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := policy.DetermineCorrectionDecision(
				domain.KindImplFeedback,
				domain.SeverityMedium,
				"", // no explicit action — let default logic run
				domain.FailureTypeExecutionFailure,
				0, // no recurrence
				domain.CorrectionMetadata{},
				tt.snapshot, domain.DefaultRoutingPolicy(),
			)

			if got.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", got.Action, tt.wantAction)
			}
			if got.RoutingMode != tt.wantMode {
				t.Errorf("RoutingMode = %q, want %q", got.RoutingMode, tt.wantMode)
			}
			if got.EscalationReason != tt.wantReason {
				t.Errorf("EscalationReason = %q, want %q", got.EscalationReason, tt.wantReason)
			}
			if got.RetryAllowed != nil && *got.RetryAllowed != tt.wantRetryAllowed {
				t.Errorf("RetryAllowed = %v, want %v", *got.RetryAllowed, tt.wantRetryAllowed)
			}
		})
	}
}

// --- C-2: Owner loop detection tests ---

func TestDetectOwnerLoop(t *testing.T) {
	tests := []struct {
		name    string
		history []string
		want    bool
	}{
		{
			name:    "ping-pong detected",
			history: []string{"sightjack", "paintress", "sightjack", "paintress"},
			want:    true,
		},
		{
			name:    "ping-pong with prefix",
			history: []string{"amadeus", "sightjack", "paintress", "sightjack", "paintress"},
			want:    true,
		},
		{
			name:    "same agent repeated is not ping-pong",
			history: []string{"paintress", "paintress", "paintress", "paintress"},
			want:    false,
		},
		{
			name:    "short history returns false",
			history: []string{"sightjack", "paintress"},
			want:    false,
		},
		{
			name:    "nil history returns false",
			history: nil,
			want:    false,
		},
		{
			name:    "no pattern in last 4",
			history: []string{"sightjack", "paintress", "amadeus", "sightjack"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := policy.DetectOwnerLoop(tt.history); got != tt.want {
				t.Errorf("DetectOwnerLoop() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetermineCorrectionDecision_EscalatesOnOwnerLoop(t *testing.T) {
	got := policy.DetermineCorrectionDecision(
		domain.KindImplFeedback,
		domain.SeverityMedium,
		"",
		domain.FailureTypeExecutionFailure,
		0,
		domain.CorrectionMetadata{
			OwnerHistory: []string{"sightjack", "paintress", "sightjack", "paintress"},
		},
		domain.ActiveProviderState(), domain.DefaultRoutingPolicy(),
	)

	if got.Action != domain.ActionEscalate {
		t.Fatalf("Action = %q, want %q", got.Action, domain.ActionEscalate)
	}
	if got.EscalationReason != "owner-loop-detected" {
		t.Fatalf("EscalationReason = %q, want owner-loop-detected", got.EscalationReason)
	}
}
