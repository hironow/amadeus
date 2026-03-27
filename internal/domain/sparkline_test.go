package domain

// white-box-reason: tests Sparkline pure function which accesses unexported sparklineRunes slice

import "testing"

func TestSparkline_EmptyInput(t *testing.T) {
	// given
	var values []float64

	// when
	result := Sparkline(values)

	// then
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestSparkline_SingleValue(t *testing.T) {
	// given
	values := []float64{42.0}

	// when
	result := Sparkline(values)

	// then
	// single value => rng==0 => idx = len/2 = 4 => '▅'
	expected := "▅"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSparkline_Ascending(t *testing.T) {
	// given
	values := []float64{0, 1, 2, 3, 4, 5, 6, 7}

	// when
	result := Sparkline(values)

	// then
	expected := "▁▂▃▄▅▆▇█"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSparkline_Descending(t *testing.T) {
	// given
	values := []float64{7, 6, 5, 4, 3, 2, 1, 0}

	// when
	result := Sparkline(values)

	// then
	expected := "█▇▆▅▄▃▂▁"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSparkline_AllEqual(t *testing.T) {
	// given
	values := []float64{5, 5, 5}

	// when
	result := Sparkline(values)

	// then
	// all equal => rng==0 => idx = len/2 = 4 => '▅' for each
	expected := "▅▅▅"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSparkline_MixedValues(t *testing.T) {
	// given
	values := []float64{10, 50, 30, 90, 0}

	// when
	result := Sparkline(values)

	// then
	if len([]rune(result)) != 5 {
		t.Errorf("expected 5 runes, got %d", len([]rune(result)))
	}
	runes := []rune(result)
	// 0 is min (▁), 90 is max (█)
	if runes[4] != '▁' {
		t.Errorf("expected min value to map to ▁, got %c", runes[4])
	}
	if runes[3] != '█' {
		t.Errorf("expected max value to map to █, got %c", runes[3])
	}
}
