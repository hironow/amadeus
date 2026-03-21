package session

// white-box-reason: session internals: tests CollectRepeatedViolations and loadRecentCheckResults

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

func TestCollectRepeatedViolations_ReturnsViolationsAboveThreshold(t *testing.T) {
	// given: 3 recent check results all scoring high on adr_integrity
	results := []domain.CheckResult{
		{
			CheckedAt:  time.Now().Add(-3 * 24 * time.Hour),
			Divergence: 40,
			Axes: map[domain.Axis]domain.AxisScore{
				domain.AxisADR: {Score: 60, Details: "ADR-003 violated"},
				domain.AxisDoD: {Score: 10, Details: "ok"},
			},
		},
		{
			CheckedAt:  time.Now().Add(-2 * 24 * time.Hour),
			Divergence: 45,
			Axes: map[domain.Axis]domain.AxisScore{
				domain.AxisADR: {Score: 65, Details: "ADR-003 still violated"},
				domain.AxisDoD: {Score: 5, Details: "ok"},
			},
		},
		{
			CheckedAt:  time.Now().Add(-1 * 24 * time.Hour),
			Divergence: 50,
			Axes: map[domain.Axis]domain.AxisScore{
				domain.AxisADR: {Score: 70, Details: "ADR-003 persists"},
				domain.AxisDoD: {Score: 8, Details: "ok"},
			},
		},
	}

	// when
	violations := CollectRepeatedViolations(results)

	// then
	if len(violations) == 0 {
		t.Fatal("expected at least 1 repeated violation for persistently high adr_integrity")
	}
	found := false
	for _, v := range violations {
		if v.Axis == string(domain.AxisADR) {
			found = true
			if v.Count != 3 {
				t.Errorf("expected count 3 for adr_integrity, got %d", v.Count)
			}
		}
	}
	if !found {
		t.Errorf("expected adr_integrity in repeated violations, got %v", violations)
	}
}

func TestCollectRepeatedViolations_EmptyResultsReturnsNil(t *testing.T) {
	// when
	violations := CollectRepeatedViolations(nil)

	// then
	if violations != nil {
		t.Errorf("expected nil for empty results, got %v", violations)
	}
}

func TestCollectRepeatedViolations_LowScoreNotRepeated(t *testing.T) {
	// given: all axes scoring low (no violations)
	results := []domain.CheckResult{
		{
			Axes: map[domain.Axis]domain.AxisScore{
				domain.AxisADR: {Score: 10, Details: "ok"},
			},
		},
		{
			Axes: map[domain.Axis]domain.AxisScore{
				domain.AxisADR: {Score: 5, Details: "ok"},
			},
		},
	}

	// when
	violations := CollectRepeatedViolations(results)

	// then
	if len(violations) != 0 {
		t.Errorf("expected no violations for low scores, got %v", violations)
	}
}

func TestLoadRecentCheckResults_ReadsFromRunDir(t *testing.T) {
	// given: a state dir with a recent-checks.json file
	stateDir := t.TempDir()
	runDir := filepath.Join(stateDir, ".run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}

	results := []domain.CheckResult{
		{
			CheckedAt:  time.Now().Add(-1 * 24 * time.Hour),
			Divergence: 42,
			Axes: map[domain.Axis]domain.AxisScore{
				domain.AxisADR: {Score: 60, Details: "violation"},
			},
		},
	}
	data, err := json.Marshal(results)
	if err != nil {
		t.Fatal(err)
	}
	recentPath := filepath.Join(runDir, "recent_checks.json")
	if err := os.WriteFile(recentPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	loaded, err := loadRecentCheckResults(stateDir)

	// then
	if err != nil {
		t.Fatalf("loadRecentCheckResults: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 result, got %d", len(loaded))
	}
	if loaded[0].Divergence != 42 {
		t.Errorf("expected divergence 42, got %f", loaded[0].Divergence)
	}
}

func TestLoadRecentCheckResults_MissingFileReturnsNil(t *testing.T) {
	// given: state dir with no recent_checks.json
	stateDir := t.TempDir()

	// when
	loaded, err := loadRecentCheckResults(stateDir)

	// then
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if loaded != nil {
		t.Errorf("expected nil for missing file, got %v", loaded)
	}
}
