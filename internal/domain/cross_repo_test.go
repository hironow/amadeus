package domain

import (
	"testing"
	"time"
)

func TestComputeEcosystemScore_EmptySnapshots(t *testing.T) {
	// given
	var snapshots []ToolSnapshot

	// when
	score := ComputeEcosystemScore(snapshots)

	// then
	if score != 0.0 {
		t.Errorf("expected 0.0, got %f", score)
	}
}

func TestComputeEcosystemScore_AllUnavailable(t *testing.T) {
	// given
	snapshots := []ToolSnapshot{
		{Tool: ToolPhonewave, Divergence: 0.5, Available: false},
		{Tool: ToolSightjack, Divergence: 0.3, Available: false},
	}

	// when
	score := ComputeEcosystemScore(snapshots)

	// then
	if score != 0.0 {
		t.Errorf("expected 0.0 when all unavailable, got %f", score)
	}
}

func TestComputeEcosystemScore_PartialAvailability(t *testing.T) {
	// given
	snapshots := []ToolSnapshot{
		{Tool: ToolPhonewave, Divergence: 0.0, Available: true},
		{Tool: ToolSightjack, Divergence: 0.0, Available: false},
		{Tool: ToolAmadeus, Divergence: 0.4, Available: true},
	}

	// when
	score := ComputeEcosystemScore(snapshots)

	// then — average of available: (0.0 + 0.4) / 2 = 0.2
	if score != 0.2 {
		t.Errorf("expected 0.2, got %f", score)
	}
}

func TestComputeEcosystemScore_AllAvailable(t *testing.T) {
	// given
	snapshots := []ToolSnapshot{
		{Tool: ToolPhonewave, Divergence: 0.0, Available: true},
		{Tool: ToolSightjack, Divergence: 0.0, Available: true},
		{Tool: ToolPaintress, Divergence: 0.0, Available: true},
		{Tool: ToolAmadeus, Divergence: 0.4, Available: true},
	}

	// when
	score := ComputeEcosystemScore(snapshots)

	// then — (0.0 + 0.0 + 0.0 + 0.4) / 4 = 0.1
	if score != 0.1 {
		t.Errorf("expected 0.1, got %f", score)
	}
}

func TestMaxSeverityAcrossTools_AllLow(t *testing.T) {
	// given
	snapshots := []ToolSnapshot{
		{Tool: ToolPhonewave, Severity: SeverityLow, Available: true},
		{Tool: ToolAmadeus, Severity: SeverityLow, Available: true},
	}

	// when
	sev := MaxSeverityAcrossTools(snapshots)

	// then
	if sev != SeverityLow {
		t.Errorf("expected low, got %s", sev)
	}
}

func TestMaxSeverityAcrossTools_OneHigh(t *testing.T) {
	// given
	snapshots := []ToolSnapshot{
		{Tool: ToolPhonewave, Severity: SeverityLow, Available: true},
		{Tool: ToolAmadeus, Severity: SeverityHigh, Available: true},
	}

	// when
	sev := MaxSeverityAcrossTools(snapshots)

	// then
	if sev != SeverityHigh {
		t.Errorf("expected high, got %s", sev)
	}
}

func TestMaxSeverityAcrossTools_Mixed(t *testing.T) {
	// given
	snapshots := []ToolSnapshot{
		{Tool: ToolPhonewave, Severity: SeverityLow, Available: true},
		{Tool: ToolSightjack, Severity: SeverityMedium, Available: true},
		{Tool: ToolPaintress, Severity: SeverityLow, Available: true},
	}

	// when
	sev := MaxSeverityAcrossTools(snapshots)

	// then
	if sev != SeverityMedium {
		t.Errorf("expected medium, got %s", sev)
	}
}

func TestMaxSeverityAcrossTools_UnavailableHighIgnored(t *testing.T) {
	// given
	snapshots := []ToolSnapshot{
		{Tool: ToolPhonewave, Severity: SeverityHigh, Available: false},
		{Tool: ToolAmadeus, Severity: SeverityLow, Available: true},
	}

	// when
	sev := MaxSeverityAcrossTools(snapshots)

	// then — unavailable tool's severity is ignored
	if sev != SeverityLow {
		t.Errorf("expected low (unavailable high ignored), got %s", sev)
	}
}

func TestMaxSeverityAcrossTools_Empty(t *testing.T) {
	// given
	var snapshots []ToolSnapshot

	// when
	sev := MaxSeverityAcrossTools(snapshots)

	// then
	if sev != SeverityLow {
		t.Errorf("expected low for empty, got %s", sev)
	}
}

func TestToolStateDirMapping(t *testing.T) {
	// given/when/then — verify all tools have correct state dir mappings
	tests := []struct {
		tool    ToolName
		wantDir string
	}{
		{ToolPhonewave, ".phonewave"},
		{ToolSightjack, ".siren"},
		{ToolPaintress, ".expedition"},
		{ToolAmadeus, ".gate"},
	}
	for _, tt := range tests {
		got := ToolStateDir(tt.tool)
		if got != tt.wantDir {
			t.Errorf("ToolStateDir(%s) = %q, want %q", tt.tool, got, tt.wantDir)
		}
	}
}

func TestToolStateDirUnknown(t *testing.T) {
	// given
	unknown := ToolName("unknown")

	// when
	got := ToolStateDir(unknown)

	// then
	if got != "" {
		t.Errorf("expected empty string for unknown tool, got %q", got)
	}
}

func TestNewCrossRepoSnapshot(t *testing.T) {
	// given
	now := time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)
	snapshots := []ToolSnapshot{
		{Tool: ToolPhonewave, Divergence: 0.0, Severity: SeverityLow, Available: true},
		{Tool: ToolAmadeus, Divergence: 0.22, Severity: SeverityLow, Available: true},
	}

	// when
	result := NewCrossRepoSnapshot(snapshots, now)

	// then
	if result.EcosystemScore != 0.11 {
		t.Errorf("expected ecosystem score 0.11, got %f", result.EcosystemScore)
	}
	if result.MaxSeverity != SeverityLow {
		t.Errorf("expected severity low, got %s", result.MaxSeverity)
	}
	if result.GeneratedAt != now {
		t.Errorf("expected generated_at to match")
	}
	if len(result.Snapshots) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(result.Snapshots))
	}
}
