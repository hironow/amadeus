package domain_test

import (
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

func TestAnalyzeConvergence_NoTargets(t *testing.T) {
	// given: D-Mails without targets
	dmails := []domain.DMail{
		{Name: "feedback-001", Metadata: map[string]string{"created_at": "2026-02-20T12:00:00Z"}},
	}
	cfg := domain.ConvergenceConfig{WindowDays: 14, Threshold: 3}
	now := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)

	// when
	alerts := domain.AnalyzeConvergence(dmails, cfg, now)

	// then
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestAnalyzeConvergence_BelowThreshold(t *testing.T) {
	// given: 2 D-Mails targeting same area (threshold=3)
	now := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)
	dmails := []domain.DMail{
		{Name: "feedback-001", Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-02-20T12:00:00Z"}},
		{Name: "feedback-002", Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-02-21T12:00:00Z"}},
	}
	cfg := domain.ConvergenceConfig{WindowDays: 14, Threshold: 3}

	// when
	alerts := domain.AnalyzeConvergence(dmails, cfg, now)

	// then
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts (below threshold), got %d", len(alerts))
	}
}

func TestAnalyzeConvergence_MeetsThreshold_MediumSeverity(t *testing.T) {
	// given: 3 D-Mails targeting same area (threshold=3)
	now := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)
	dmails := []domain.DMail{
		{Name: "feedback-001", Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-02-18T12:00:00Z"}},
		{Name: "feedback-002", Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-02-19T12:00:00Z"}},
		{Name: "feedback-003", Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-02-20T12:00:00Z"}},
	}
	cfg := domain.ConvergenceConfig{WindowDays: 14, Threshold: 3}

	// when
	alerts := domain.AnalyzeConvergence(dmails, cfg, now)

	// then
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Target != "auth/session.go" {
		t.Errorf("expected target 'auth/session.go', got %s", alerts[0].Target)
	}
	if alerts[0].Count != 3 {
		t.Errorf("expected count 3, got %d", alerts[0].Count)
	}
	if alerts[0].Severity != domain.SeverityMedium {
		t.Errorf("expected severity medium, got %s", alerts[0].Severity)
	}
}

func TestAnalyzeConvergence_DoubleThreshold_HighSeverity(t *testing.T) {
	// given: 6 D-Mails targeting same area (threshold=3, 6 >= 3*2)
	now := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)
	var dmails []domain.DMail
	for i := 0; i < 6; i++ {
		day := 15 + i
		dmails = append(dmails, domain.DMail{
			Name:    "feedback-" + string(rune('a'+i)),
			Targets: []string{"auth/session.go"},
			Metadata: map[string]string{
				"created_at": time.Date(2026, 2, day, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
			},
		})
	}
	cfg := domain.ConvergenceConfig{WindowDays: 14, Threshold: 3}

	// when
	alerts := domain.AnalyzeConvergence(dmails, cfg, now)

	// then
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Severity != domain.SeverityHigh {
		t.Errorf("expected severity high, got %s", alerts[0].Severity)
	}
}

func TestAnalyzeConvergence_OutsideWindow(t *testing.T) {
	// given: old D-Mails outside the 14-day window
	now := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)
	dmails := []domain.DMail{
		{Name: "feedback-001", Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-01-01T12:00:00Z"}},
		{Name: "feedback-002", Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-01-02T12:00:00Z"}},
		{Name: "feedback-003", Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-01-03T12:00:00Z"}},
	}
	cfg := domain.ConvergenceConfig{WindowDays: 14, Threshold: 3}

	// when
	alerts := domain.AnalyzeConvergence(dmails, cfg, now)

	// then
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts (outside window), got %d", len(alerts))
	}
}

func TestAnalyzeConvergence_MultipleTargets(t *testing.T) {
	// given: D-Mails split across two targets
	now := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)
	dmails := []domain.DMail{
		{Name: "feedback-001", Targets: []string{"auth/session.go", "api/handler.go"},
			Metadata: map[string]string{"created_at": "2026-02-18T12:00:00Z"}},
		{Name: "feedback-002", Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-02-19T12:00:00Z"}},
		{Name: "feedback-003", Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-02-20T12:00:00Z"}},
	}
	cfg := domain.ConvergenceConfig{WindowDays: 14, Threshold: 3}

	// when
	alerts := domain.AnalyzeConvergence(dmails, cfg, now)

	// then: only auth/session.go should trigger (3 hits), not api/handler.go (1 hit)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Target != "auth/session.go" {
		t.Errorf("expected target 'auth/session.go', got %s", alerts[0].Target)
	}
}

func TestAnalyzeConvergence_FirstSeenLastSeen(t *testing.T) {
	now := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)
	dmails := []domain.DMail{
		{Name: "feedback-001", Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-02-18T12:00:00Z"}},
		{Name: "feedback-002", Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-02-20T12:00:00Z"}},
		{Name: "feedback-003", Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-02-19T12:00:00Z"}},
	}
	cfg := domain.ConvergenceConfig{WindowDays: 14, Threshold: 3}

	alerts := domain.AnalyzeConvergence(dmails, cfg, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	expected := time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC)
	if !alerts[0].FirstSeen.Equal(expected) {
		t.Errorf("expected first_seen %v, got %v", expected, alerts[0].FirstSeen)
	}
	expectedLast := time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC)
	if !alerts[0].LastSeen.Equal(expectedLast) {
		t.Errorf("expected last_seen %v, got %v", expectedLast, alerts[0].LastSeen)
	}
}

func TestGenerateConvergenceDMails_OnlyHigh(t *testing.T) {
	// given: one medium, one high alert
	alerts := []domain.ConvergenceAlert{
		{Target: "auth/session.go", Count: 3, Window: 14, Severity: domain.SeverityMedium,
			DMails: []string{"feedback-001", "feedback-002", "feedback-003"}},
		{Target: "api/handler.go", Count: 6, Window: 14, Severity: domain.SeverityHigh,
			DMails: []string{"feedback-004", "feedback-005", "feedback-006", "feedback-007", "feedback-008", "feedback-009"}},
	}

	// when
	dmails := domain.GenerateConvergenceDMails(alerts)

	// then: only HIGH severity generates D-Mails
	if len(dmails) != 1 {
		t.Fatalf("expected 1 D-Mail, got %d", len(dmails))
	}
	if dmails[0].Kind != domain.KindConvergence {
		t.Errorf("expected kind convergence, got %s", dmails[0].Kind)
	}
	if dmails[0].Severity != domain.SeverityHigh {
		t.Errorf("expected severity high, got %s", dmails[0].Severity)
	}
	if len(dmails[0].Targets) != 1 || dmails[0].Targets[0] != "api/handler.go" {
		t.Errorf("expected target 'api/handler.go', got %v", dmails[0].Targets)
	}
	if dmails[0].SchemaVersion != domain.DMailSchemaVersion {
		t.Errorf("expected schema version %q, got %q", domain.DMailSchemaVersion, dmails[0].SchemaVersion)
	}
}

func TestGenerateConvergenceDMails_Empty(t *testing.T) {
	// given: no alerts
	dmails := domain.GenerateConvergenceDMails(nil)

	// then
	if len(dmails) != 0 {
		t.Errorf("expected 0 D-Mails, got %d", len(dmails))
	}
}

func TestAnalyzeConvergence_NoMetadata(t *testing.T) {
	// given: D-Mail without created_at metadata
	now := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)
	dmails := []domain.DMail{
		{Name: "feedback-001", Targets: []string{"auth/session.go"}},
		{Name: "feedback-002", Targets: []string{"auth/session.go"}},
		{Name: "feedback-003", Targets: []string{"auth/session.go"}},
	}
	cfg := domain.ConvergenceConfig{WindowDays: 14, Threshold: 3}

	// when
	alerts := domain.AnalyzeConvergence(dmails, cfg, now)

	// then: no alerts (no valid timestamps)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts (no metadata), got %d", len(alerts))
	}
}

func TestAnalyzeConvergence_CustomEscalationMultiplier(t *testing.T) {
	// given: 4 D-Mails, threshold=2, escalation_multiplier=3 → HIGH at 6
	now := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)
	var dmails []domain.DMail
	for i := 0; i < 4; i++ {
		dmails = append(dmails, domain.DMail{
			Name:    "feedback-" + string(rune('a'+i)),
			Targets: []string{"auth/session.go"},
			Metadata: map[string]string{
				"created_at": time.Date(2026, 2, 15+i, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
			},
		})
	}
	cfg := domain.ConvergenceConfig{
		WindowDays:           14,
		Threshold:            2,
		EscalationMultiplier: 3,
	}

	// when
	alerts := domain.AnalyzeConvergence(dmails, cfg, now)

	// then: 4 hits >= threshold=2 → alert exists, but 4 < 2*3=6 → MEDIUM
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Severity != domain.SeverityMedium {
		t.Errorf("expected MEDIUM (4 < 2*3=6), got %s", alerts[0].Severity)
	}
}

func TestAnalyzeConvergence_DefaultEscalationMultiplier(t *testing.T) {
	// given: EscalationMultiplier=0 (zero value) should default to 2
	now := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)
	var dmails []domain.DMail
	for i := 0; i < 6; i++ {
		dmails = append(dmails, domain.DMail{
			Name:    "feedback-" + string(rune('a'+i)),
			Targets: []string{"auth/session.go"},
			Metadata: map[string]string{
				"created_at": time.Date(2026, 2, 15+i, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
			},
		})
	}
	cfg := domain.ConvergenceConfig{
		WindowDays: 14,
		Threshold:  3,
		// EscalationMultiplier: 0 → default to 2
	}

	// when
	alerts := domain.AnalyzeConvergence(dmails, cfg, now)

	// then: 6 >= 3*2=6 → HIGH
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Severity != domain.SeverityHigh {
		t.Errorf("expected HIGH (6 >= 3*2=6 with default multiplier), got %s", alerts[0].Severity)
	}
}

func TestAnalyzeConvergence_ExcludesConvergenceDMails(t *testing.T) {
	// given: 2 feedback D-Mails + 1 convergence D-Mail targeting same area
	// Without filtering, count=3 would meet threshold=3 and trigger an alert.
	// With filtering, count=2 should NOT trigger (below threshold).
	now := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)
	dmails := []domain.DMail{
		{Name: "feedback-001", Kind: domain.KindFeedback, Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-02-19T12:00:00Z"}},
		{Name: "feedback-002", Kind: domain.KindFeedback, Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-02-20T12:00:00Z"}},
		{Name: "convergence-001", Kind: domain.KindConvergence, Targets: []string{"auth/session.go"},
			Metadata: map[string]string{"created_at": "2026-02-21T12:00:00Z", "convergence_for": "feedback-001,feedback-002"}},
	}
	cfg := domain.ConvergenceConfig{WindowDays: 14, Threshold: 3}

	// when
	alerts := domain.AnalyzeConvergence(dmails, cfg, now)

	// then: convergence D-Mail should be excluded, so only 2 hits (below threshold)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts (convergence D-Mails excluded), got %d", len(alerts))
	}
}
