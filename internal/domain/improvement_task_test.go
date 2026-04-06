package domain_test

import (
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

func TestNewImprovementTask(t *testing.T) {
	// given
	now := time.Now()

	// when
	task := domain.NewImprovementTask(
		"dmail-spec-001",
		"paintress",
		"retry with corrected prompt",
		domain.FailureTypeExecutionFailure,
		domain.SeverityMedium,
		30*time.Minute,
	)

	// then
	if task.ID == "" {
		t.Error("expected non-empty ID")
	}
	if task.SourceEvent != "dmail-spec-001" {
		t.Errorf("SourceEvent = %q, want dmail-spec-001", task.SourceEvent)
	}
	if task.TargetAgent != "paintress" {
		t.Errorf("TargetAgent = %q, want paintress", task.TargetAgent)
	}
	if task.SuggestedAction != "retry with corrected prompt" {
		t.Errorf("SuggestedAction = %q", task.SuggestedAction)
	}
	if task.FailureType != domain.FailureTypeExecutionFailure {
		t.Errorf("FailureType = %q", task.FailureType)
	}
	if task.Severity != domain.SeverityMedium {
		t.Errorf("Severity = %q", task.Severity)
	}
	if task.CreatedAt.Before(now) {
		t.Error("CreatedAt should be >= test start time")
	}
	if task.ExpiresAt.Before(task.CreatedAt) {
		t.Error("ExpiresAt should be after CreatedAt")
	}
}

func TestImprovementTask_Expired(t *testing.T) {
	// given — task with 0 TTL (already expired)
	task := domain.NewImprovementTask("ev", "agent", "action", domain.FailureTypeNone, domain.SeverityLow, 0)

	// then
	if !task.Expired() {
		t.Error("expected expired with 0 TTL")
	}
}

func TestImprovementTask_NotExpired(t *testing.T) {
	// given
	task := domain.NewImprovementTask("ev", "agent", "action", domain.FailureTypeNone, domain.SeverityLow, time.Hour)

	// then
	if task.Expired() {
		t.Error("expected not expired with 1h TTL")
	}
}
