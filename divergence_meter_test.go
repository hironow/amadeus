package amadeus

import (
	"testing"
)

func TestDivergenceMeter_ProcessResponse(t *testing.T) {
	meter := &DivergenceMeter{
		Config: DefaultConfig(),
	}
	resp := ClaudeResponse{
		Axes: map[Axis]AxisScore{
			AxisADR:        {Score: 15, Details: "minor"},
			AxisDoD:        {Score: 20, Details: "edge case"},
			AxisDependency: {Score: 10, Details: "clean"},
			AxisImplicit:   {Score: 5, Details: "naming"},
		},
		DMails: []ClaudeDMailCandidate{
			{Description: "ADR-003", Detail: "violation"},
		},
		Reasoning: "Minor tensions",
	}
	result := meter.ProcessResponse(resp)
	if !almostEqual(result.Divergence.Internal, 14.5) {
		t.Errorf("expected internal 14.5, got %f", result.Divergence.Internal)
	}
	if result.Divergence.Severity != SeverityLow {
		t.Errorf("expected low severity, got %s", result.Divergence.Severity)
	}
	if len(result.DMailCandidates) != 1 {
		t.Errorf("expected 1 D-Mail candidate, got %d", len(result.DMailCandidates))
	}
}

func TestDivergenceMeter_ProcessResponse_PassesImpactRadius(t *testing.T) {
	// given
	meter := &DivergenceMeter{Config: DefaultConfig()}
	resp := ClaudeResponse{
		Axes: map[Axis]AxisScore{
			AxisADR:        {Score: 0, Details: "ok"},
			AxisDoD:        {Score: 0, Details: "ok"},
			AxisDependency: {Score: 0, Details: "ok"},
			AxisImplicit:   {Score: 0, Details: "ok"},
		},
		DMails:    []ClaudeDMailCandidate{},
		Reasoning: "clean",
		ImpactRadius: []ImpactEntry{
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
	meter := &DivergenceMeter{Config: DefaultConfig()}
	resp := ClaudeResponse{
		Axes: map[Axis]AxisScore{
			AxisADR:        {Score: 0, Details: "ok"},
			AxisDoD:        {Score: 0, Details: "ok"},
			AxisDependency: {Score: 0, Details: "ok"},
			AxisImplicit:   {Score: 0, Details: "ok"},
		},
		DMails:    []ClaudeDMailCandidate{},
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
	meter := &DivergenceMeter{
		Config: DefaultConfig(),
	}
	resp := ClaudeResponse{
		Axes: map[Axis]AxisScore{
			AxisADR:        {Score: 70, Details: "major violation"},
			AxisDoD:        {Score: 50, Details: "failing"},
			AxisDependency: {Score: 40, Details: "broken"},
			AxisImplicit:   {Score: 30, Details: "messy"},
		},
		DMails:    []ClaudeDMailCandidate{},
		Reasoning: "Serious issues",
	}
	result := meter.ProcessResponse(resp)
	if result.Divergence.Severity != SeverityHigh {
		t.Errorf("expected high severity, got %s", result.Divergence.Severity)
	}
}
