package domain

import (
	"strconv"
	"strings"
	"time"
)

type ProviderState string

const (
	ProviderStateActive   ProviderState = "active"
	ProviderStateWaiting  ProviderState = "waiting"
	ProviderStateDegraded ProviderState = "degraded"
	ProviderStatePaused   ProviderState = "paused"
)

const (
	ProviderReasonProbe                = "probe"
	ProviderReasonRateLimit            = "rate_limit"
	ProviderReasonServerError          = "server_error"
	ProviderReasonDeliveryRetryBackoff = "delivery_retry_backoff"
)

const (
	ResumeConditionProviderAvailable = "provider-available"
	ResumeConditionBackoffElapses    = "backoff-elapses"
	ResumeConditionProviderReset     = "provider-reset-window"
	ResumeConditionProbeSucceeds     = "probe-succeeds"
)

const (
	MetadataProviderState       = "provider_state"
	MetadataProviderReason      = "provider_reason"
	MetadataProviderRetryBudget = "provider_retry_budget"
	MetadataProviderResumeAt    = "provider_resume_at"
	MetadataProviderResumeWhen  = "provider_resume_when"
)

type ProviderStateSnapshot struct {
	State           ProviderState
	Reason          string
	RetryBudget     int
	ResumeAt        time.Time
	ResumeCondition string
}

func NormalizeProviderState(state ProviderState) ProviderState {
	switch ProviderState(strings.ToLower(string(state))) {
	case ProviderStateActive:
		return ProviderStateActive
	case ProviderStateWaiting:
		return ProviderStateWaiting
	case ProviderStateDegraded:
		return ProviderStateDegraded
	case ProviderStatePaused:
		return ProviderStatePaused
	default:
		return state
	}
}

func IsKnownProviderState(state ProviderState) bool {
	switch NormalizeProviderState(state) {
	case ProviderStateActive, ProviderStateWaiting, ProviderStateDegraded, ProviderStatePaused:
		return true
	default:
		return false
	}
}

func ActiveProviderState() ProviderStateSnapshot {
	return ProviderStateSnapshot{
		State:           ProviderStateActive,
		RetryBudget:     1,
		ResumeCondition: ResumeConditionProviderAvailable,
	}
}

func (s ProviderStateSnapshot) ApplyMetadata(meta map[string]string) map[string]string {
	cp := make(map[string]string, len(meta)+5)
	for k, v := range meta {
		cp[k] = v
	}
	state := s.State
	if state == "" {
		state = ProviderStateActive
	} else {
		state = NormalizeProviderState(state)
	}
	cp[MetadataProviderState] = string(state)
	cp[MetadataProviderRetryBudget] = strconv.Itoa(s.RetryBudget)
	if s.Reason != "" {
		cp[MetadataProviderReason] = s.Reason
	}
	if !s.ResumeAt.IsZero() {
		cp[MetadataProviderResumeAt] = s.ResumeAt.UTC().Format(time.RFC3339)
	}
	if s.ResumeCondition != "" {
		cp[MetadataProviderResumeWhen] = s.ResumeCondition
	}
	return cp
}
