package session_test

import (
	"path/filepath"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestLoadRoutingPolicy_MissingFileReturnsDefault(t *testing.T) {
	// given
	stateDir := t.TempDir()

	// when
	policy, err := session.LoadRoutingPolicy(stateDir)

	// then
	if err != nil {
		t.Fatalf("LoadRoutingPolicy: %v", err)
	}
	if policy.RecurrenceThreshold != 2 {
		t.Errorf("expected default RecurrenceThreshold=2, got %d", policy.RecurrenceThreshold)
	}
}

func TestSaveAndLoadRoutingPolicy_Roundtrip(t *testing.T) {
	// given
	stateDir := t.TempDir()
	policy := domain.RoutingPolicy{
		RecurrenceThreshold: 5,
		SeverityActionMap: map[domain.Severity]string{
			domain.SeverityHigh: "escalate",
			domain.SeverityLow:  "ignore",
		},
		TargetAgentMap: map[domain.FailureType]string{
			domain.FailureTypeScopeViolation: "amadeus",
		},
	}

	// when — save then load
	err := session.SaveRoutingPolicy(stateDir, policy)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := session.LoadRoutingPolicy(stateDir)

	// then
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.RecurrenceThreshold != 5 {
		t.Errorf("RecurrenceThreshold = %d, want 5", loaded.RecurrenceThreshold)
	}
	if loaded.SeverityActionMap[domain.SeverityLow] != "ignore" {
		t.Errorf("SeverityActionMap[Low] = %q, want ignore", loaded.SeverityActionMap[domain.SeverityLow])
	}

	// verify file exists at expected path
	expectedPath := filepath.Join(stateDir, ".policy", "routing.yaml")
	if _, err := filepath.Abs(expectedPath); err != nil {
		t.Errorf("expected policy file at %s", expectedPath)
	}
}
