package domain_test

import (
	"math"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestCalcDivergence_AllZero(t *testing.T) {
	axes := map[domain.Axis]domain.AxisScore{
		domain.AxisADR:        {Score: 0},
		domain.AxisDoD:        {Score: 0},
		domain.AxisDependency: {Score: 0},
		domain.AxisImplicit:   {Score: 0},
	}
	result := domain.CalcDivergence(axes, domain.DefaultWeights())
	if !almostEqual(result.Value, 0.0) {
		t.Errorf("expected 0.000000, got %f", result.Value)
	}
	if !almostEqual(result.Internal, 0.0) {
		t.Errorf("expected internal 0.0, got %f", result.Internal)
	}
}

func TestCalcDivergence_MaxDeviation(t *testing.T) {
	axes := map[domain.Axis]domain.AxisScore{
		domain.AxisADR:        {Score: 100},
		domain.AxisDoD:        {Score: 100},
		domain.AxisDependency: {Score: 100},
		domain.AxisImplicit:   {Score: 100},
	}
	result := domain.CalcDivergence(axes, domain.DefaultWeights())
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
	axes := map[domain.Axis]domain.AxisScore{
		domain.AxisADR:        {Score: 15, Details: "ADR-003 minor tension"},
		domain.AxisDoD:        {Score: 20, Details: "Issue #42 edge case"},
		domain.AxisDependency: {Score: 10, Details: "clean"},
		domain.AxisImplicit:   {Score: 5, Details: "naming drift in cart"},
	}
	result := domain.CalcDivergence(axes, domain.DefaultWeights())
	if !almostEqual(result.Internal, 14.5) {
		t.Errorf("expected internal 14.5, got %f", result.Internal)
	}
	if !almostEqual(result.Value, 0.145) {
		t.Errorf("expected 0.145000, got %f", result.Value)
	}
}

func TestDetermineSeverity_Low(t *testing.T) {
	result := domain.DivergenceResult{Internal: 10.0, Value: 0.10, Axes: map[domain.Axis]domain.AxisScore{
		domain.AxisADR: {Score: 10}, domain.AxisDoD: {Score: 10}, domain.AxisDependency: {Score: 10}, domain.AxisImplicit: {Score: 10},
	}}
	sev := domain.DetermineSeverity(result, domain.DefaultThresholds())
	if sev.Severity != domain.SeverityLow {
		t.Errorf("expected low, got %s", sev.Severity)
	}
	if sev.Overridden {
		t.Error("expected no override")
	}
}

func TestDetermineSeverity_Medium(t *testing.T) {
	result := domain.DivergenceResult{Internal: 35.0, Value: 0.35, Axes: map[domain.Axis]domain.AxisScore{
		domain.AxisADR: {Score: 30}, domain.AxisDoD: {Score: 30}, domain.AxisDependency: {Score: 30}, domain.AxisImplicit: {Score: 30},
	}}
	sev := domain.DetermineSeverity(result, domain.DefaultThresholds())
	if sev.Severity != domain.SeverityMedium {
		t.Errorf("expected medium, got %s", sev.Severity)
	}
}

func TestDetermineSeverity_High(t *testing.T) {
	result := domain.DivergenceResult{Internal: 60.0, Value: 0.60, Axes: map[domain.Axis]domain.AxisScore{
		domain.AxisADR: {Score: 50}, domain.AxisDoD: {Score: 50}, domain.AxisDependency: {Score: 50}, domain.AxisImplicit: {Score: 50},
	}}
	sev := domain.DetermineSeverity(result, domain.DefaultThresholds())
	if sev.Severity != domain.SeverityHigh {
		t.Errorf("expected high, got %s", sev.Severity)
	}
}

func TestDetermineSeverity_ADROverrideForceHigh(t *testing.T) {
	// Total divergence is LOW (internal=24) but ADR axis=60 forces HIGH
	result := domain.DivergenceResult{Internal: 24.0, Value: 0.24, Axes: map[domain.Axis]domain.AxisScore{
		domain.AxisADR: {Score: 60}, domain.AxisDoD: {Score: 0}, domain.AxisDependency: {Score: 0}, domain.AxisImplicit: {Score: 0},
	}}
	sev := domain.DetermineSeverity(result, domain.DefaultThresholds())
	if sev.Severity != domain.SeverityHigh {
		t.Errorf("expected high (ADR override), got %s", sev.Severity)
	}
	if !sev.Overridden {
		t.Error("expected override flag to be true")
	}
}

func TestDetermineSeverity_DoDOverrideForceHigh(t *testing.T) {
	result := domain.DivergenceResult{Internal: 21.0, Value: 0.21, Axes: map[domain.Axis]domain.AxisScore{
		domain.AxisADR: {Score: 0}, domain.AxisDoD: {Score: 70}, domain.AxisDependency: {Score: 0}, domain.AxisImplicit: {Score: 0},
	}}
	sev := domain.DetermineSeverity(result, domain.DefaultThresholds())
	if sev.Severity != domain.SeverityHigh {
		t.Errorf("expected high (DoD override), got %s", sev.Severity)
	}
	if !sev.Overridden {
		t.Error("expected override flag to be true")
	}
}

func TestDetermineSeverity_DepOverrideForceMedium(t *testing.T) {
	result := domain.DivergenceResult{Internal: 16.0, Value: 0.16, Axes: map[domain.Axis]domain.AxisScore{
		domain.AxisADR: {Score: 0}, domain.AxisDoD: {Score: 0}, domain.AxisDependency: {Score: 80}, domain.AxisImplicit: {Score: 0},
	}}
	sev := domain.DetermineSeverity(result, domain.DefaultThresholds())
	if sev.Severity != domain.SeverityMedium {
		t.Errorf("expected medium (Dep override), got %s", sev.Severity)
	}
	if !sev.Overridden {
		t.Error("expected override flag to be true")
	}
}

func TestNormalizeSeverity(t *testing.T) {
	tests := []struct {
		name     string
		input    domain.Severity
		expected domain.Severity
	}{
		{"lowercase low", domain.SeverityLow, domain.SeverityLow},
		{"lowercase medium", domain.SeverityMedium, domain.SeverityMedium},
		{"lowercase high", domain.SeverityHigh, domain.SeverityHigh},
		{"uppercase LOW", domain.Severity("LOW"), domain.SeverityLow},
		{"uppercase MEDIUM", domain.Severity("MEDIUM"), domain.SeverityMedium},
		{"uppercase HIGH", domain.Severity("HIGH"), domain.SeverityHigh},
		{"mixed case High", domain.Severity("High"), domain.SeverityHigh},
		{"mixed case Medium", domain.Severity("Medium"), domain.SeverityMedium},
		{"empty string", domain.Severity(""), domain.Severity("")},
		{"unrecognized", domain.Severity("critical"), domain.Severity("critical")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.NormalizeSeverity(tt.input)
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
			got := domain.FormatDivergence(tt.internal)
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
			got := domain.FormatDelta(tt.current, tt.previous)
			if got != tt.expected {
				t.Errorf("FormatDelta(%f, %f) = %q, want %q", tt.current, tt.previous, got, tt.expected)
			}
		})
	}
}

func TestDivergenceMeter_ProcessResponse(t *testing.T) {
	meter := &domain.DivergenceMeter{
		Config: domain.DefaultConfig(),
	}
	resp := domain.ClaudeResponse{
		Axes: map[domain.Axis]domain.AxisScore{
			domain.AxisADR:        {Score: 15, Details: "minor"},
			domain.AxisDoD:        {Score: 20, Details: "edge case"},
			domain.AxisDependency: {Score: 10, Details: "clean"},
			domain.AxisImplicit:   {Score: 5, Details: "naming"},
		},
		DMails: []domain.ClaudeDMailCandidate{
			{Description: "ADR-003", Detail: "violation"},
		},
		Reasoning: "Minor tensions",
	}
	result := meter.ProcessResponse(resp)
	if !almostEqual(result.Divergence.Internal, 14.5) {
		t.Errorf("expected internal 14.5, got %f", result.Divergence.Internal)
	}
	if result.Divergence.Severity != domain.SeverityLow {
		t.Errorf("expected low severity, got %s", result.Divergence.Severity)
	}
	if len(result.DMailCandidates) != 1 {
		t.Errorf("expected 1 D-Mail candidate, got %d", len(result.DMailCandidates))
	}
}

func TestDivergenceMeter_ProcessResponse_PassesImpactRadius(t *testing.T) {
	// given
	meter := &domain.DivergenceMeter{Config: domain.DefaultConfig()}
	resp := domain.ClaudeResponse{
		Axes: map[domain.Axis]domain.AxisScore{
			domain.AxisADR:        {Score: 0, Details: "ok"},
			domain.AxisDoD:        {Score: 0, Details: "ok"},
			domain.AxisDependency: {Score: 0, Details: "ok"},
			domain.AxisImplicit:   {Score: 0, Details: "ok"},
		},
		DMails:    []domain.ClaudeDMailCandidate{},
		Reasoning: "clean",
		ImpactRadius: []domain.ImpactEntry{
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
	meter := &domain.DivergenceMeter{Config: domain.DefaultConfig()}
	resp := domain.ClaudeResponse{
		Axes: map[domain.Axis]domain.AxisScore{
			domain.AxisADR:        {Score: 0, Details: "ok"},
			domain.AxisDoD:        {Score: 0, Details: "ok"},
			domain.AxisDependency: {Score: 0, Details: "ok"},
			domain.AxisImplicit:   {Score: 0, Details: "ok"},
		},
		DMails:    []domain.ClaudeDMailCandidate{},
		Reasoning: "clean",
	}

	// when
	result := meter.ProcessResponse(resp)

	// then
	if result.ImpactRadius != nil {
		t.Errorf("expected nil impact radius, got %v", result.ImpactRadius)
	}
}

func TestClassifyByAxes_DesignDominant(t *testing.T) {
	axes := map[domain.Axis]domain.AxisScore{
		domain.AxisADR:        {Score: 80},
		domain.AxisDoD:        {Score: 10},
		domain.AxisDependency: {Score: 60},
		domain.AxisImplicit:   {Score: 5},
	}
	category := domain.ClassifyByAxes(axes, domain.DefaultWeights())
	if category != "design" {
		t.Errorf("expected design, got %s", category)
	}
}

func TestClassifyByAxes_ImplementationDominant(t *testing.T) {
	axes := map[domain.Axis]domain.AxisScore{
		domain.AxisADR:        {Score: 5},
		domain.AxisDoD:        {Score: 80},
		domain.AxisDependency: {Score: 10},
		domain.AxisImplicit:   {Score: 70},
	}
	category := domain.ClassifyByAxes(axes, domain.DefaultWeights())
	if category != "implementation" {
		t.Errorf("expected implementation, got %s", category)
	}
}

func TestClassifyByAxes_TieGoesToDesign(t *testing.T) {
	axes := map[domain.Axis]domain.AxisScore{
		domain.AxisADR:        {Score: 50},
		domain.AxisDoD:        {Score: 50},
		domain.AxisDependency: {Score: 50},
		domain.AxisImplicit:   {Score: 50},
	}
	weights := domain.Weights{
		ADRIntegrity: 0.25, DoDFulfillment: 0.25,
		DependencyIntegrity: 0.25, ImplicitConstraints: 0.25,
	}
	category := domain.ClassifyByAxes(axes, weights)
	if category != "design" {
		t.Errorf("expected design on tie, got %s", category)
	}
}

func TestResolveFeedbackKinds_BothDesign(t *testing.T) {
	kinds := domain.ResolveFeedbackKinds("design", "design")
	if len(kinds) != 1 || kinds[0] != domain.KindDesignFeedback {
		t.Errorf("expected [design-feedback], got %v", kinds)
	}
}

func TestResolveFeedbackKinds_BothImpl(t *testing.T) {
	kinds := domain.ResolveFeedbackKinds("implementation", "implementation")
	if len(kinds) != 1 || kinds[0] != domain.KindImplFeedback {
		t.Errorf("expected [implementation-feedback], got %v", kinds)
	}
}

func TestResolveFeedbackKinds_Disagreement(t *testing.T) {
	kinds := domain.ResolveFeedbackKinds("design", "implementation")
	if len(kinds) != 2 {
		t.Fatalf("expected 2 kinds on disagreement, got %d", len(kinds))
	}
}

func TestDivergenceMeter_ProcessResponse_HighSeverity(t *testing.T) {
	meter := &domain.DivergenceMeter{
		Config: domain.DefaultConfig(),
	}
	resp := domain.ClaudeResponse{
		Axes: map[domain.Axis]domain.AxisScore{
			domain.AxisADR:        {Score: 70, Details: "major violation"},
			domain.AxisDoD:        {Score: 50, Details: "failing"},
			domain.AxisDependency: {Score: 40, Details: "broken"},
			domain.AxisImplicit:   {Score: 30, Details: "messy"},
		},
		DMails:    []domain.ClaudeDMailCandidate{},
		Reasoning: "Serious issues",
	}
	result := meter.ProcessResponse(resp)
	if result.Divergence.Severity != domain.SeverityHigh {
		t.Errorf("expected high severity, got %s", result.Divergence.Severity)
	}
}
