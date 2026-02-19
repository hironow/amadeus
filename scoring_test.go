package amadeus

import (
	"math"
	"testing"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestCalcDivergence_AllZero(t *testing.T) {
	axes := map[Axis]AxisScore{
		AxisADR:        {Score: 0},
		AxisDoD:        {Score: 0},
		AxisDependency: {Score: 0},
		AxisImplicit:   {Score: 0},
	}
	result := CalcDivergence(axes, DefaultWeights())
	if !almostEqual(result.Value, 0.0) {
		t.Errorf("expected 0.000000, got %f", result.Value)
	}
	if !almostEqual(result.Internal, 0.0) {
		t.Errorf("expected internal 0.0, got %f", result.Internal)
	}
}

func TestCalcDivergence_MaxDeviation(t *testing.T) {
	axes := map[Axis]AxisScore{
		AxisADR:        {Score: 100},
		AxisDoD:        {Score: 100},
		AxisDependency: {Score: 100},
		AxisImplicit:   {Score: 100},
	}
	result := CalcDivergence(axes, DefaultWeights())
	if !almostEqual(result.Value, 1.0) {
		t.Errorf("expected 1.000000, got %f", result.Value)
	}
	if !almostEqual(result.Internal, 100.0) {
		t.Errorf("expected internal 100.0, got %f", result.Internal)
	}
}

func TestCalcDivergence_WeightedSum(t *testing.T) {
	// From architecture doc example:
	// ADR=15, DoD=20, Dep=10, Implicit=5
	// Internal = 15*0.4 + 20*0.3 + 10*0.2 + 5*0.1 = 6+6+2+0.5 = 14.5
	axes := map[Axis]AxisScore{
		AxisADR:        {Score: 15, Details: "ADR-003 minor tension"},
		AxisDoD:        {Score: 20, Details: "Issue #42 edge case"},
		AxisDependency: {Score: 10, Details: "clean"},
		AxisImplicit:   {Score: 5, Details: "naming drift in cart"},
	}
	result := CalcDivergence(axes, DefaultWeights())
	if !almostEqual(result.Internal, 14.5) {
		t.Errorf("expected internal 14.5, got %f", result.Internal)
	}
	if !almostEqual(result.Value, 0.145) {
		t.Errorf("expected 0.145000, got %f", result.Value)
	}
}

func TestDetermineSeverity_Low(t *testing.T) {
	result := DivergenceResult{Internal: 10.0, Value: 0.10, Axes: map[Axis]AxisScore{
		AxisADR: {Score: 10}, AxisDoD: {Score: 10}, AxisDependency: {Score: 10}, AxisImplicit: {Score: 10},
	}}
	sev := DetermineSeverity(result, DefaultThresholds())
	if sev.Severity != SeverityLow {
		t.Errorf("expected LOW, got %s", sev.Severity)
	}
	if sev.Overridden {
		t.Error("expected no override")
	}
}

func TestDetermineSeverity_Medium(t *testing.T) {
	result := DivergenceResult{Internal: 35.0, Value: 0.35, Axes: map[Axis]AxisScore{
		AxisADR: {Score: 30}, AxisDoD: {Score: 30}, AxisDependency: {Score: 30}, AxisImplicit: {Score: 30},
	}}
	sev := DetermineSeverity(result, DefaultThresholds())
	if sev.Severity != SeverityMedium {
		t.Errorf("expected MEDIUM, got %s", sev.Severity)
	}
}

func TestDetermineSeverity_High(t *testing.T) {
	result := DivergenceResult{Internal: 60.0, Value: 0.60, Axes: map[Axis]AxisScore{
		AxisADR: {Score: 50}, AxisDoD: {Score: 50}, AxisDependency: {Score: 50}, AxisImplicit: {Score: 50},
	}}
	sev := DetermineSeverity(result, DefaultThresholds())
	if sev.Severity != SeverityHigh {
		t.Errorf("expected HIGH, got %s", sev.Severity)
	}
}

func TestDetermineSeverity_ADROverrideForceHigh(t *testing.T) {
	// Total divergence is LOW (internal=24) but ADR axis=60 forces HIGH
	result := DivergenceResult{Internal: 24.0, Value: 0.24, Axes: map[Axis]AxisScore{
		AxisADR: {Score: 60}, AxisDoD: {Score: 0}, AxisDependency: {Score: 0}, AxisImplicit: {Score: 0},
	}}
	sev := DetermineSeverity(result, DefaultThresholds())
	if sev.Severity != SeverityHigh {
		t.Errorf("expected HIGH (ADR override), got %s", sev.Severity)
	}
	if !sev.Overridden {
		t.Error("expected override flag to be true")
	}
}

func TestDetermineSeverity_DoDOverrideForceHigh(t *testing.T) {
	result := DivergenceResult{Internal: 21.0, Value: 0.21, Axes: map[Axis]AxisScore{
		AxisADR: {Score: 0}, AxisDoD: {Score: 70}, AxisDependency: {Score: 0}, AxisImplicit: {Score: 0},
	}}
	sev := DetermineSeverity(result, DefaultThresholds())
	if sev.Severity != SeverityHigh {
		t.Errorf("expected HIGH (DoD override), got %s", sev.Severity)
	}
	if !sev.Overridden {
		t.Error("expected override flag to be true")
	}
}

func TestDetermineSeverity_DepOverrideForceMedium(t *testing.T) {
	result := DivergenceResult{Internal: 16.0, Value: 0.16, Axes: map[Axis]AxisScore{
		AxisADR: {Score: 0}, AxisDoD: {Score: 0}, AxisDependency: {Score: 80}, AxisImplicit: {Score: 0},
	}}
	sev := DetermineSeverity(result, DefaultThresholds())
	if sev.Severity != SeverityMedium {
		t.Errorf("expected MEDIUM (Dep override), got %s", sev.Severity)
	}
	if !sev.Overridden {
		t.Error("expected override flag to be true")
	}
}

func TestFormatDivergence(t *testing.T) {
	tests := []struct {
		name     string
		internal float64
		expected string
	}{
		{"zero", 0.0, "0.000000"},
		{"max", 100.0, "1.000000"},
		{"example from doc", 14.5, "0.145000"},
		{"small value", 0.5, "0.005000"},
		{"boundary low", 25.0, "0.250000"},
		{"boundary high", 50.0, "0.500000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDivergence(tt.internal)
			if got != tt.expected {
				t.Errorf("FormatDivergence(%f) = %q, want %q", tt.internal, got, tt.expected)
			}
		})
	}
}

func TestFormatDelta(t *testing.T) {
	tests := []struct {
		name     string
		current  float64
		previous float64
		expected string
	}{
		{"positive delta", 0.145, 0.133, "+0.012000"},
		{"negative delta", 0.10, 0.15, "-0.050000"},
		{"zero delta", 0.25, 0.25, "+0.000000"},
		{"first check (no previous)", 0.145, 0.0, "+0.145000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDelta(tt.current, tt.previous)
			if got != tt.expected {
				t.Errorf("FormatDelta(%f, %f) = %q, want %q", tt.current, tt.previous, got, tt.expected)
			}
		})
	}
}
