package amadeus_test

import (
	"math"
	"testing"

	"github.com/hironow/amadeus"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestCalcDivergence_AllZero(t *testing.T) {
	axes := map[amadeus.Axis]amadeus.AxisScore{
		amadeus.AxisADR:        {Score: 0},
		amadeus.AxisDoD:        {Score: 0},
		amadeus.AxisDependency: {Score: 0},
		amadeus.AxisImplicit:   {Score: 0},
	}
	result := amadeus.CalcDivergence(axes, amadeus.DefaultWeights())
	if !almostEqual(result.Value, 0.0) {
		t.Errorf("expected 0.000000, got %f", result.Value)
	}
	if !almostEqual(result.Internal, 0.0) {
		t.Errorf("expected internal 0.0, got %f", result.Internal)
	}
}

func TestCalcDivergence_MaxDeviation(t *testing.T) {
	axes := map[amadeus.Axis]amadeus.AxisScore{
		amadeus.AxisADR:        {Score: 100},
		amadeus.AxisDoD:        {Score: 100},
		amadeus.AxisDependency: {Score: 100},
		amadeus.AxisImplicit:   {Score: 100},
	}
	result := amadeus.CalcDivergence(axes, amadeus.DefaultWeights())
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
	axes := map[amadeus.Axis]amadeus.AxisScore{
		amadeus.AxisADR:        {Score: 15, Details: "ADR-003 minor tension"},
		amadeus.AxisDoD:        {Score: 20, Details: "Issue #42 edge case"},
		amadeus.AxisDependency: {Score: 10, Details: "clean"},
		amadeus.AxisImplicit:   {Score: 5, Details: "naming drift in cart"},
	}
	result := amadeus.CalcDivergence(axes, amadeus.DefaultWeights())
	if !almostEqual(result.Internal, 14.5) {
		t.Errorf("expected internal 14.5, got %f", result.Internal)
	}
	if !almostEqual(result.Value, 0.145) {
		t.Errorf("expected 0.145000, got %f", result.Value)
	}
}

func TestDetermineSeverity_Low(t *testing.T) {
	result := amadeus.DivergenceResult{Internal: 10.0, Value: 0.10, Axes: map[amadeus.Axis]amadeus.AxisScore{
		amadeus.AxisADR: {Score: 10}, amadeus.AxisDoD: {Score: 10}, amadeus.AxisDependency: {Score: 10}, amadeus.AxisImplicit: {Score: 10},
	}}
	sev := amadeus.DetermineSeverity(result, amadeus.DefaultThresholds())
	if sev.Severity != amadeus.SeverityLow {
		t.Errorf("expected low, got %s", sev.Severity)
	}
	if sev.Overridden {
		t.Error("expected no override")
	}
}

func TestDetermineSeverity_Medium(t *testing.T) {
	result := amadeus.DivergenceResult{Internal: 35.0, Value: 0.35, Axes: map[amadeus.Axis]amadeus.AxisScore{
		amadeus.AxisADR: {Score: 30}, amadeus.AxisDoD: {Score: 30}, amadeus.AxisDependency: {Score: 30}, amadeus.AxisImplicit: {Score: 30},
	}}
	sev := amadeus.DetermineSeverity(result, amadeus.DefaultThresholds())
	if sev.Severity != amadeus.SeverityMedium {
		t.Errorf("expected medium, got %s", sev.Severity)
	}
}

func TestDetermineSeverity_High(t *testing.T) {
	result := amadeus.DivergenceResult{Internal: 60.0, Value: 0.60, Axes: map[amadeus.Axis]amadeus.AxisScore{
		amadeus.AxisADR: {Score: 50}, amadeus.AxisDoD: {Score: 50}, amadeus.AxisDependency: {Score: 50}, amadeus.AxisImplicit: {Score: 50},
	}}
	sev := amadeus.DetermineSeverity(result, amadeus.DefaultThresholds())
	if sev.Severity != amadeus.SeverityHigh {
		t.Errorf("expected high, got %s", sev.Severity)
	}
}

func TestDetermineSeverity_ADROverrideForceHigh(t *testing.T) {
	// Total divergence is LOW (internal=24) but ADR axis=60 forces HIGH
	result := amadeus.DivergenceResult{Internal: 24.0, Value: 0.24, Axes: map[amadeus.Axis]amadeus.AxisScore{
		amadeus.AxisADR: {Score: 60}, amadeus.AxisDoD: {Score: 0}, amadeus.AxisDependency: {Score: 0}, amadeus.AxisImplicit: {Score: 0},
	}}
	sev := amadeus.DetermineSeverity(result, amadeus.DefaultThresholds())
	if sev.Severity != amadeus.SeverityHigh {
		t.Errorf("expected high (ADR override), got %s", sev.Severity)
	}
	if !sev.Overridden {
		t.Error("expected override flag to be true")
	}
}

func TestDetermineSeverity_DoDOverrideForceHigh(t *testing.T) {
	result := amadeus.DivergenceResult{Internal: 21.0, Value: 0.21, Axes: map[amadeus.Axis]amadeus.AxisScore{
		amadeus.AxisADR: {Score: 0}, amadeus.AxisDoD: {Score: 70}, amadeus.AxisDependency: {Score: 0}, amadeus.AxisImplicit: {Score: 0},
	}}
	sev := amadeus.DetermineSeverity(result, amadeus.DefaultThresholds())
	if sev.Severity != amadeus.SeverityHigh {
		t.Errorf("expected high (DoD override), got %s", sev.Severity)
	}
	if !sev.Overridden {
		t.Error("expected override flag to be true")
	}
}

func TestDetermineSeverity_DepOverrideForceMedium(t *testing.T) {
	result := amadeus.DivergenceResult{Internal: 16.0, Value: 0.16, Axes: map[amadeus.Axis]amadeus.AxisScore{
		amadeus.AxisADR: {Score: 0}, amadeus.AxisDoD: {Score: 0}, amadeus.AxisDependency: {Score: 80}, amadeus.AxisImplicit: {Score: 0},
	}}
	sev := amadeus.DetermineSeverity(result, amadeus.DefaultThresholds())
	if sev.Severity != amadeus.SeverityMedium {
		t.Errorf("expected medium (Dep override), got %s", sev.Severity)
	}
	if !sev.Overridden {
		t.Error("expected override flag to be true")
	}
}

func TestNormalizeSeverity(t *testing.T) {
	tests := []struct {
		name     string
		input    amadeus.Severity
		expected amadeus.Severity
	}{
		{"lowercase low", amadeus.SeverityLow, amadeus.SeverityLow},
		{"lowercase medium", amadeus.SeverityMedium, amadeus.SeverityMedium},
		{"lowercase high", amadeus.SeverityHigh, amadeus.SeverityHigh},
		{"uppercase LOW", amadeus.Severity("LOW"), amadeus.SeverityLow},
		{"uppercase MEDIUM", amadeus.Severity("MEDIUM"), amadeus.SeverityMedium},
		{"uppercase HIGH", amadeus.Severity("HIGH"), amadeus.SeverityHigh},
		{"mixed case High", amadeus.Severity("High"), amadeus.SeverityHigh},
		{"mixed case Medium", amadeus.Severity("Medium"), amadeus.SeverityMedium},
		{"empty string", amadeus.Severity(""), amadeus.Severity("")},
		{"unrecognized", amadeus.Severity("critical"), amadeus.Severity("critical")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := amadeus.NormalizeSeverity(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeSeverity(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
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
			got := amadeus.FormatDivergence(tt.internal)
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
			got := amadeus.FormatDelta(tt.current, tt.previous)
			if got != tt.expected {
				t.Errorf("FormatDelta(%f, %f) = %q, want %q", tt.current, tt.previous, got, tt.expected)
			}
		})
	}
}

func TestDivergenceMeter_ProcessResponse(t *testing.T) {
	meter := &amadeus.DivergenceMeter{
		Config: amadeus.DefaultConfig(),
	}
	resp := amadeus.ClaudeResponse{
		Axes: map[amadeus.Axis]amadeus.AxisScore{
			amadeus.AxisADR:        {Score: 15, Details: "minor"},
			amadeus.AxisDoD:        {Score: 20, Details: "edge case"},
			amadeus.AxisDependency: {Score: 10, Details: "clean"},
			amadeus.AxisImplicit:   {Score: 5, Details: "naming"},
		},
		DMails: []amadeus.ClaudeDMailCandidate{
			{Description: "ADR-003", Detail: "violation"},
		},
		Reasoning: "Minor tensions",
	}
	result := meter.ProcessResponse(resp)
	if !almostEqual(result.Divergence.Internal, 14.5) {
		t.Errorf("expected internal 14.5, got %f", result.Divergence.Internal)
	}
	if result.Divergence.Severity != amadeus.SeverityLow {
		t.Errorf("expected low severity, got %s", result.Divergence.Severity)
	}
	if len(result.DMailCandidates) != 1 {
		t.Errorf("expected 1 D-Mail candidate, got %d", len(result.DMailCandidates))
	}
}

func TestDivergenceMeter_ProcessResponse_PassesImpactRadius(t *testing.T) {
	// given
	meter := &amadeus.DivergenceMeter{Config: amadeus.DefaultConfig()}
	resp := amadeus.ClaudeResponse{
		Axes: map[amadeus.Axis]amadeus.AxisScore{
			amadeus.AxisADR:        {Score: 0, Details: "ok"},
			amadeus.AxisDoD:        {Score: 0, Details: "ok"},
			amadeus.AxisDependency: {Score: 0, Details: "ok"},
			amadeus.AxisImplicit:   {Score: 0, Details: "ok"},
		},
		DMails:    []amadeus.ClaudeDMailCandidate{},
		Reasoning: "clean",
		ImpactRadius: []amadeus.ImpactEntry{
			{Area: "auth/session.go", Impact: "direct", Detail: "changed"},
			{Area: "api/handler.go", Impact: "indirect", Detail: "calls auth"},
		},
	}

	// when
	result := meter.ProcessResponse(resp)

	// then
	if len(result.ImpactRadius) != 2 {
		t.Fatalf("expected 2 impact entries, got %d", len(result.ImpactRadius))
	}
	if result.ImpactRadius[0].Area != "auth/session.go" {
		t.Errorf("expected area 'auth/session.go', got %q", result.ImpactRadius[0].Area)
	}
	if result.ImpactRadius[1].Impact != "indirect" {
		t.Errorf("expected impact 'indirect', got %q", result.ImpactRadius[1].Impact)
	}
}

func TestDivergenceMeter_ProcessResponse_NilImpactRadius(t *testing.T) {
	// given: ClaudeResponse without ImpactRadius
	meter := &amadeus.DivergenceMeter{Config: amadeus.DefaultConfig()}
	resp := amadeus.ClaudeResponse{
		Axes: map[amadeus.Axis]amadeus.AxisScore{
			amadeus.AxisADR:        {Score: 0, Details: "ok"},
			amadeus.AxisDoD:        {Score: 0, Details: "ok"},
			amadeus.AxisDependency: {Score: 0, Details: "ok"},
			amadeus.AxisImplicit:   {Score: 0, Details: "ok"},
		},
		DMails:    []amadeus.ClaudeDMailCandidate{},
		Reasoning: "clean",
	}

	// when
	result := meter.ProcessResponse(resp)

	// then
	if result.ImpactRadius != nil {
		t.Errorf("expected nil impact radius, got %v", result.ImpactRadius)
	}
}

func TestDivergenceMeter_ProcessResponse_HighSeverity(t *testing.T) {
	meter := &amadeus.DivergenceMeter{
		Config: amadeus.DefaultConfig(),
	}
	resp := amadeus.ClaudeResponse{
		Axes: map[amadeus.Axis]amadeus.AxisScore{
			amadeus.AxisADR:        {Score: 70, Details: "major violation"},
			amadeus.AxisDoD:        {Score: 50, Details: "failing"},
			amadeus.AxisDependency: {Score: 40, Details: "broken"},
			amadeus.AxisImplicit:   {Score: 30, Details: "messy"},
		},
		DMails:    []amadeus.ClaudeDMailCandidate{},
		Reasoning: "Serious issues",
	}
	result := meter.ProcessResponse(resp)
	if result.Divergence.Severity != amadeus.SeverityHigh {
		t.Errorf("expected high severity, got %s", result.Divergence.Severity)
	}
}
