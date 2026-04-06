package policy_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness/policy"
)

func TestEvaluateExhaustion(t *testing.T) {
	tests := []struct {
		name     string
		snapshot domain.ProviderStateSnapshot
		want     policy.ExhaustionAction
	}{
		{
			name: "active with zero budget pauses",
			snapshot: domain.ProviderStateSnapshot{
				State:       domain.ProviderStateActive,
				RetryBudget: 0,
			},
			want: policy.ExhaustionPause,
		},
		{
			name: "waiting with zero budget waits",
			snapshot: domain.ProviderStateSnapshot{
				State:           domain.ProviderStateWaiting,
				RetryBudget:     0,
				ResumeCondition: domain.ResumeConditionBackoffElapses,
			},
			want: policy.ExhaustionWait,
		},
		{
			name: "degraded with any budget pauses",
			snapshot: domain.ProviderStateSnapshot{
				State:           domain.ProviderStateDegraded,
				RetryBudget:     1,
				ResumeCondition: domain.ResumeConditionProbeSucceeds,
			},
			want: policy.ExhaustionPause,
		},
		{
			name: "degraded with zero budget pauses",
			snapshot: domain.ProviderStateSnapshot{
				State:       domain.ProviderStateDegraded,
				RetryBudget: 0,
			},
			want: policy.ExhaustionPause,
		},
		{
			name: "paused with zero budget aborts",
			snapshot: domain.ProviderStateSnapshot{
				State:           domain.ProviderStatePaused,
				RetryBudget:     0,
				ResumeCondition: domain.ResumeConditionProviderReset,
			},
			want: policy.ExhaustionAbort,
		},
		{
			name: "paused with positive budget still aborts",
			snapshot: domain.ProviderStateSnapshot{
				State:           domain.ProviderStatePaused,
				RetryBudget:     3,
				ResumeCondition: domain.ResumeConditionProviderReset,
			},
			want: policy.ExhaustionAbort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got := policy.EvaluateExhaustion(tt.snapshot)

			// then
			if got != tt.want {
				t.Errorf("EvaluateExhaustion() = %v, want %v", got, tt.want)
			}
		})
	}
}
