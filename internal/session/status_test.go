package session_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestStatus_EmptyState(t *testing.T) {
	// given — fresh directory with no events, no inbox, no archive
	baseDir := t.TempDir()

	// when
	report := session.Status(context.Background(), baseDir, &domain.NopLogger{})

	// then
	if report.CheckCount != 0 {
		t.Errorf("expected CheckCount=0, got %d", report.CheckCount)
	}
	if report.InboxCount != 0 {
		t.Errorf("expected InboxCount=0, got %d", report.InboxCount)
	}
	if report.ArchiveCount != 0 {
		t.Errorf("expected ArchiveCount=0, got %d", report.ArchiveCount)
	}
	if report.SuccessRate != 0.0 {
		t.Errorf("expected SuccessRate=0.0, got %f", report.SuccessRate)
	}
	if report.Divergence != 0.0 {
		t.Errorf("expected Divergence=0.0, got %f", report.Divergence)
	}
	if report.Convergences != 0 {
		t.Errorf("expected Convergences=0, got %d", report.Convergences)
	}
	if !report.LastCheck.IsZero() {
		t.Errorf("expected LastCheck to be zero, got %v", report.LastCheck)
	}
}

func TestStatus_WithMailDirs(t *testing.T) {
	// given — create inbox and archive with files
	baseDir := t.TempDir()
	inboxDir := filepath.Join(baseDir, "inbox")
	archiveDir := filepath.Join(baseDir, "archive")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create 2 inbox files
	for _, name := range []string{"feedback-001.md", "feedback-002.md"} {
		if err := os.WriteFile(filepath.Join(inboxDir, name), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Create 3 archive files
	for _, name := range []string{"feedback-001.md", "feedback-002.md", "convergence-001.md"} {
		if err := os.WriteFile(filepath.Join(archiveDir, name), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// when
	report := session.Status(context.Background(), baseDir, &domain.NopLogger{})

	// then
	if report.InboxCount != 2 {
		t.Errorf("expected InboxCount=2, got %d", report.InboxCount)
	}
	if report.ArchiveCount != 3 {
		t.Errorf("expected ArchiveCount=3, got %d", report.ArchiveCount)
	}
}

func TestStatus_WithEvents(t *testing.T) {
	// given — create event store with check events
	baseDir := t.TempDir()
	eventsDir := filepath.Join(baseDir, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	store := session.NewEventStore(baseDir, &domain.NopLogger{})

	now := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

	// Create 2 check.completed events: one clean, one with drift
	cleanResult := domain.CheckResult{
		CheckedAt:  now,
		Commit:     "abc123",
		Type:       domain.CheckTypeDiff,
		Divergence: 0.05,
		DMails:     nil,
	}
	cleanEv, err := domain.NewEvent(domain.EventCheckCompleted, domain.CheckCompletedData{
		Result: cleanResult,
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	driftResult := domain.CheckResult{
		CheckedAt:  now.Add(time.Hour),
		Commit:     "def456",
		Type:       domain.CheckTypeDiff,
		Divergence: 0.35,
		DMails:     []string{"feedback-001"},
	}
	driftEv, err := domain.NewEvent(domain.EventCheckCompleted, domain.CheckCompletedData{
		Result: driftResult,
	}, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	// Create a convergence event
	convergenceEv, err := domain.NewEvent(domain.EventConvergenceDetected, domain.ConvergenceDetectedData{
		Alert: domain.ConvergenceAlert{
			Target:   "internal/session",
			Count:    3,
			Window:   30,
			DMails:   []string{"feedback-001", "feedback-002", "feedback-003"},
			Severity: domain.SeverityHigh,
		},
	}, now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Append(cleanEv, driftEv, convergenceEv); err != nil {
		t.Fatal(err)
	}

	// when
	report := session.Status(context.Background(), baseDir, &domain.NopLogger{})

	// then
	if report.CheckCount != 2 {
		t.Errorf("expected CheckCount=2, got %d", report.CheckCount)
	}
	if report.SuccessRate != 0.5 {
		t.Errorf("expected SuccessRate=0.5, got %f", report.SuccessRate)
	}
	if report.Divergence != 0.35 {
		t.Errorf("expected Divergence=0.35, got %f", report.Divergence)
	}
	if !report.LastCheck.Equal(now.Add(time.Hour)) {
		t.Errorf("expected LastCheck=%v, got %v", now.Add(time.Hour), report.LastCheck)
	}
	if report.Convergences != 1 {
		t.Errorf("expected Convergences=1, got %d", report.Convergences)
	}
}

func TestStatus_FormatText(t *testing.T) {
	// given
	report := domain.StatusReport{
		LastCheck:    time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC),
		Divergence:   0.15,
		CheckCount:   8,
		InboxCount:   2,
		ArchiveCount: 12,
		SuccessRate:  0.875,
		Convergences: 1,
	}

	// when
	text := report.FormatText()

	// then — verify key lines are present
	expected := []string{
		"amadeus status:",
		"Last check:",
		"Divergence:",
		"Checks:",
		"Success rate:",
		"Inbox:",
		"Archive:",
		"Convergences:",
		"87.5%",
		"0.15",
		"8 total",
		"2 pending",
		"12 processed",
		"1 active",
	}
	for _, s := range expected {
		if !strings.Contains(text, s) {
			t.Errorf("expected output to contain %q, got:\n%s", s, text)
		}
	}
}

func TestStatus_FormatText_NoChecks(t *testing.T) {
	// given — zero-value report
	report := domain.StatusReport{}

	// when
	text := report.FormatText()

	// then
	if !strings.Contains(text, "no checks yet") {
		t.Errorf("expected 'no checks yet' for zero time, got:\n%s", text)
	}
	if !strings.Contains(text, "no events") {
		t.Errorf("expected 'no events' for zero check count, got:\n%s", text)
	}
}

func TestStatus_FormatJSON(t *testing.T) {
	// given
	report := domain.StatusReport{
		LastCheck:    time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC),
		Divergence:   0.15,
		CheckCount:   8,
		InboxCount:   2,
		ArchiveCount: 12,
		SuccessRate:  0.875,
		Convergences: 1,
	}

	// when
	data := report.FormatJSON()

	// then — verify it's valid JSON with expected fields
	var parsed map[string]any
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, data)
	}
	if parsed["check_count"] != float64(8) {
		t.Errorf("expected check_count=8, got %v", parsed["check_count"])
	}
	if parsed["inbox_count"] != float64(2) {
		t.Errorf("expected inbox_count=2, got %v", parsed["inbox_count"])
	}
	if parsed["divergence"] != 0.15 {
		t.Errorf("expected divergence=0.15, got %v", parsed["divergence"])
	}
	if parsed["success_rate"] != 0.875 {
		t.Errorf("expected success_rate=0.875, got %v", parsed["success_rate"])
	}
}
