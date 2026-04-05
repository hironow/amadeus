package domain

import (
	"strconv"
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

func ActiveProviderState() ProviderStateSnapshot {
	return ProviderStateSnapshot{
		State:           ProviderStateActive,
		RetryBudget:     1,
		ResumeCondition: "provider-available",
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
