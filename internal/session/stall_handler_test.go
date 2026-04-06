package session_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestExtractStallEscalations_NoStalls(t *testing.T) {
	// given
	dmails := []domain.DMail{
		{Name: "report-1", Kind: domain.KindReport, Description: "normal report"},
	}

	// when
	stalls := session.ExtractStallEscalations(dmails)

	// then
	if len(stalls) != 0 {
		t.Errorf("expected 0 stall escalations, got %d", len(stalls))
	}
}

func TestExtractStallEscalations_WithStall(t *testing.T) {
	// given
	dmails := []domain.DMail{
		{Name: "report-1", Kind: domain.KindReport, Description: "normal report"},
		{
			Name:        "stall-auth-w1",
			Kind:        domain.KindStallEscalation,
			Description: "Wave auth:w1 stalled: repeated structural error detected",
			Severity:    "high",
			Action:      "escalate",
			Metadata: map[string]string{
				"wave_id":           "w1",
				"cluster_name":      "auth",
				"error_fingerprint": "abc123",
				"failure_count":     "3",
				"detected_at":       "2026-04-07T10:00:00Z",
			},
		},
	}

	// when
	stalls := session.ExtractStallEscalations(dmails)

	// then
	if len(stalls) != 1 {
		t.Fatalf("expected 1 stall escalation, got %d", len(stalls))
	}
	if stalls[0].Name != "stall-auth-w1" {
		t.Errorf("stall name = %q, want %q", stalls[0].Name, "stall-auth-w1")
	}
	if stalls[0].Metadata["wave_id"] != "w1" {
		t.Errorf("wave_id = %q, want %q", stalls[0].Metadata["wave_id"], "w1")
	}
}
