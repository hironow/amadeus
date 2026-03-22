package session

// white-box-reason: session internals: tests AnalyzeDivergenceTrend logic

import (
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

func TestAnalyzeDivergenceTrend_WorsenigTrend(t *testing.T) {
	// given: divergence consistently increasing
	results := []domain.CheckResult{
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
	if trend.Class != domain.DivergenceTrendWorsening {
		t.Errorf("expected Worsening, got %q", trend.Class)
	}
	if trend.Delta <= 0 {
		t.Errorf("expected positive delta for worsening, got %f", trend.Delta)
	}
}

func TestAnalyzeDivergenceTrend_ImprovingTrend(t *testing.T) {
	// given: divergence consistently decreasing
	results := []domain.CheckResult{
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
	if trend.Class != domain.DivergenceTrendImproving {
		t.Errorf("expected Improving, got %q", trend.Class)
	}
	if trend.Delta >= 0 {
		t.Errorf("expected negative delta for improving, got %f", trend.Delta)
	}
}

func TestAnalyzeDivergenceTrend_StableTrend(t *testing.T) {
	// given: divergence oscillating within a narrow band
	results := []domain.CheckResult{
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
	if trend.Class != domain.DivergenceTrendStable {
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
	trend := AnalyzeDivergenceTrend([]domain.CheckResult{
		{Divergence: 50},
	})

	// then
	if trend != nil {
		t.Errorf("expected nil for single result (no trend possible), got %v", trend)
	}
}
