package domain

// white-box-reason: tests CollectRepeatedViolations and AnalyzeDivergenceTrend pure functions

import (
	"testing"
	"time"
)

func TestCollectRepeatedViolations_ReturnsViolationsAboveThreshold(t *testing.T) {
	// given: 3 recent check results all scoring high on adr_integrity
	results := []CheckResult{
		{
			CheckedAt:  time.Now().Add(-3 * 24 * time.Hour),
			Divergence: 40,
			Axes: map[Axis]AxisScore{
				AxisADR: {Score: 60, Details: "ADR-003 violated"},
				AxisDoD: {Score: 10, Details: "ok"},
			},
		},
		{
			CheckedAt:  time.Now().Add(-2 * 24 * time.Hour),
			Divergence: 45,
			Axes: map[Axis]AxisScore{
				AxisADR: {Score: 65, Details: "ADR-003 still violated"},
				AxisDoD: {Score: 5, Details: "ok"},
			},
		},
		{
			CheckedAt:  time.Now().Add(-1 * 24 * time.Hour),
			Divergence: 50,
			Axes: map[Axis]AxisScore{
				AxisADR: {Score: 70, Details: "ADR-003 persists"},
				AxisDoD: {Score: 8, Details: "ok"},
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
		if v.Axis == string(AxisADR) {
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
	results := []CheckResult{
		{
			Axes: map[Axis]AxisScore{
				AxisADR: {Score: 10, Details: "ok"},
			},
		},
		{
			Axes: map[Axis]AxisScore{
				AxisADR: {Score: 5, Details: "ok"},
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

func TestAnalyzeDivergenceTrend_WorseningTrend(t *testing.T) {
	// given: divergence consistently increasing
	results := []CheckResult{
		{CheckedAt: time.Now().Add(-3 * 24 * time.Hour), Divergence: 20},
		{CheckedAt: time.Now().Add(-2 * 24 * time.Hour), Divergence: 30},
		{CheckedAt: time.Now().Add(-1 * 24 * time.Hour), Divergence: 45},
	}

	// when
	trend := AnalyzeDivergenceTrend(results)

	// then
	if trend == nil {
		t.Fatal("expected non-nil trend for worsening divergence")
	}
	if trend.Class != DivergenceTrendWorsening {
		t.Errorf("expected Worsening, got %q", trend.Class)
	}
	if trend.Delta <= 0 {
		t.Errorf("expected positive delta for worsening, got %f", trend.Delta)
	}
}

func TestAnalyzeDivergenceTrend_ImprovingTrend(t *testing.T) {
	// given: divergence consistently decreasing
	results := []CheckResult{
		{CheckedAt: time.Now().Add(-3 * 24 * time.Hour), Divergence: 60},
		{CheckedAt: time.Now().Add(-2 * 24 * time.Hour), Divergence: 40},
		{CheckedAt: time.Now().Add(-1 * 24 * time.Hour), Divergence: 20},
	}

	// when
	trend := AnalyzeDivergenceTrend(results)

	// then
	if trend == nil {
		t.Fatal("expected non-nil trend for improving divergence")
	}
	if trend.Class != DivergenceTrendImproving {
		t.Errorf("expected Improving, got %q", trend.Class)
	}
	if trend.Delta >= 0 {
		t.Errorf("expected negative delta for improving, got %f", trend.Delta)
	}
}

func TestAnalyzeDivergenceTrend_StableTrend(t *testing.T) {
	// given: divergence oscillating within a narrow band
	results := []CheckResult{
		{CheckedAt: time.Now().Add(-3 * 24 * time.Hour), Divergence: 30},
		{CheckedAt: time.Now().Add(-2 * 24 * time.Hour), Divergence: 31},
		{CheckedAt: time.Now().Add(-1 * 24 * time.Hour), Divergence: 30},
	}

	// when
	trend := AnalyzeDivergenceTrend(results)

	// then
	if trend == nil {
		t.Fatal("expected non-nil trend")
	}
	if trend.Class != DivergenceTrendStable {
		t.Errorf("expected Stable, got %q", trend.Class)
	}
}

func TestAnalyzeDivergenceTrend_NilForEmptyResults(t *testing.T) {
	// when
	trend := AnalyzeDivergenceTrend(nil)

	// then
	if trend != nil {
		t.Errorf("expected nil for empty results, got %v", trend)
	}
}

func TestAnalyzeDivergenceTrend_NilForSingleResult(t *testing.T) {
	// when
	trend := AnalyzeDivergenceTrend([]CheckResult{
		{Divergence: 50},
	})

	// then
	if trend != nil {
		t.Errorf("expected nil for single result (no trend possible), got %v", trend)
	}
}
