package session_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
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

func TestHandleStallEscalations_LogsMetadata(t *testing.T) {
	// given: stall-escalation with full metadata
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	stalls := []domain.DMail{
		{
			Name:        "stall-auth-w1",
			Kind:        domain.KindStallEscalation,
			Description: "Wave auth:w1 stalled",
			Metadata: map[string]string{
				"wave_id":           "w1",
				"cluster_name":      "auth",
				"error_fingerprint": "abc123",
				"failure_count":     "3",
			},
		},
	}

	// when
	count := session.HandleStallEscalations(stalls, logger)

	// then
	if count != 1 {
		t.Errorf("expected 1 handled, got %d", count)
	}
	output := buf.String()
	for _, want := range []string{"[STALL]", "auth", "w1", "abc123", "3"} {
		if !strings.Contains(output, want) {
			t.Errorf("log output missing %q, got: %s", want, output)
		}
	}
}

func TestConsumeInbox_StallEscalation_EndToEnd(t *testing.T) {
	// given: mixed inbox with report + stall-escalation
	inbox := []domain.DMail{
		{Name: "report-1", Kind: domain.KindReport, Description: "normal report"},
		{
			Name:        "stall-billing-w3",
			Kind:        domain.KindStallEscalation,
			Description: "Wave billing:w3 stalled",
			Metadata: map[string]string{
				"wave_id":           "w3",
				"cluster_name":      "billing",
				"error_fingerprint": "def456",
				"failure_count":     "5",
			},
		},
	}

	// when: extract + handle (simulates the consumeInbox → stall pipeline)
	stalls := session.ExtractStallEscalations(inbox)
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	count := session.HandleStallEscalations(stalls, logger)

	// then
	if count != 1 {
		t.Fatalf("expected 1 stall, got %d", count)
	}
	output := buf.String()
	if !strings.Contains(output, "billing") {
		t.Errorf("expected billing cluster in log, got: %s", output)
	}
	if !strings.Contains(output, "def456") {
		t.Errorf("expected fingerprint in log, got: %s", output)
	}

	// Remaining non-stall D-Mails should still be available for downstream
	remaining := len(inbox) - len(stalls)
	if remaining != 1 {
		t.Errorf("expected 1 remaining non-stall D-Mail, got %d", remaining)
	}
}
