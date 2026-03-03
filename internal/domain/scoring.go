package domain

import (
	"fmt"
	"strings"
)

// Axis represents an evaluation axis for divergence scoring.
type Axis string

const (
	AxisADR        Axis = "adr_integrity"
	AxisDoD        Axis = "dod_fulfillment"
	AxisDependency Axis = "dependency_integrity"
	AxisImplicit   Axis = "implicit_constraints"
)

// AxisScore holds the score and details for a single evaluation axis.
type AxisScore struct {
	Score   int    `json:"score"`
	Details string `json:"details"`
}

// Weights holds the configurable weights for each evaluation axis.
type Weights struct {
	ADRIntegrity        float64 `yaml:"adr_integrity" json:"adr_integrity"`
	DoDFulfillment      float64 `yaml:"dod_fulfillment" json:"dod_fulfillment"`
	DependencyIntegrity float64 `yaml:"dependency_integrity" json:"dependency_integrity"`
	ImplicitConstraints float64 `yaml:"implicit_constraints" json:"implicit_constraints"`
}

// DefaultWeights returns the standard weights from the architecture document.
func DefaultWeights() Weights {
	return Weights{
		ADRIntegrity:        0.4,
		DoDFulfillment:      0.3,
		DependencyIntegrity: 0.2,
		ImplicitConstraints: 0.1,
	}
}

// DivergenceResult holds the complete result of a divergence calculation.
type DivergenceResult struct {
	Value      float64            `json:"divergence"`
	Internal   float64            `json:"internal"`
	Axes       map[Axis]AxisScore `json:"axes"`
	Severity   Severity           `json:"severity"`
	Overridden bool               `json:"overridden"`
}

// Severity represents the D-Mail severity tier.
type Severity string

const (
	SeverityLow    Severity = "low"
	SeverityMedium Severity = "medium"
	SeverityHigh   Severity = "high"
)

// NormalizeSeverity converts legacy uppercase severity values to lowercase.
// Returns the input unchanged if already lowercase or unrecognized.
func NormalizeSeverity(s Severity) Severity {
	switch Severity(strings.ToLower(string(s))) {
	case SeverityLow:
		return SeverityLow
	case SeverityMedium:
		return SeverityMedium
	case SeverityHigh:
		return SeverityHigh
	default:
		return s
	}
}

// Thresholds holds the severity threshold configuration.
type Thresholds struct {
	LowMax    float64 `yaml:"low_max" json:"low_max"`
	MediumMax float64 `yaml:"medium_max" json:"medium_max"`
}

// PerAxisOverride holds per-axis critical thresholds that escalate severity.
type PerAxisOverride struct {
	ADRForceHigh   int `yaml:"adr_integrity_force_high" json:"adr_integrity_force_high"`
	DoDForceHigh   int `yaml:"dod_fulfillment_force_high" json:"dod_fulfillment_force_high"`
	DepForceMedium int `yaml:"dependency_integrity_force_medium" json:"dependency_integrity_force_medium"`
}

// SeverityConfig combines thresholds and per-axis overrides.
type SeverityConfig struct {
	Thresholds      Thresholds      `yaml:"thresholds" json:"thresholds"`
	PerAxisOverride PerAxisOverride `yaml:"per_axis_override" json:"per_axis_override"`
}

// DefaultThresholds returns the standard thresholds from the architecture document.
func DefaultThresholds() SeverityConfig {
	return SeverityConfig{
		Thresholds: Thresholds{
			LowMax:    0.250000,
			MediumMax: 0.500000,
		},
		PerAxisOverride: PerAxisOverride{
			ADRForceHigh:   60,
			DoDForceHigh:   70,
			DepForceMedium: 80,
		},
	}
}

// CalcDivergence computes the weighted divergence score from axis scores.
func CalcDivergence(axes map[Axis]AxisScore, weights Weights) DivergenceResult {
	internal := float64(axes[AxisADR].Score)*weights.ADRIntegrity +
		float64(axes[AxisDoD].Score)*weights.DoDFulfillment +
		float64(axes[AxisDependency].Score)*weights.DependencyIntegrity +
		float64(axes[AxisImplicit].Score)*weights.ImplicitConstraints

	return DivergenceResult{
		Value:    internal / 100.0,
		Internal: internal,
		Axes:     axes,
	}
}

// DetermineSeverity applies threshold and per-axis override rules.
func DetermineSeverity(result DivergenceResult, config SeverityConfig) DivergenceResult {
	severity := SeverityLow
	if result.Value >= config.Thresholds.MediumMax {
		severity = SeverityHigh
	} else if result.Value >= config.Thresholds.LowMax {
		severity = SeverityMedium
	}

	overridden := false
	if result.Axes[AxisADR].Score >= config.PerAxisOverride.ADRForceHigh {
		if severity != SeverityHigh {
			overridden = true
		}
		severity = SeverityHigh
	}
	if result.Axes[AxisDoD].Score >= config.PerAxisOverride.DoDForceHigh {
		if severity != SeverityHigh {
			overridden = true
		}
		severity = SeverityHigh
	}
	if result.Axes[AxisDependency].Score >= config.PerAxisOverride.DepForceMedium {
		if severity == SeverityLow {
			overridden = true
			severity = SeverityMedium
		}
	}

	result.Severity = severity
	result.Overridden = overridden
	return result
}

// FormatDivergence converts an internal score (0-100) to 0.000000 display format.
func FormatDivergence(internal float64) string {
	return fmt.Sprintf("%f", internal/100.0)
}

// FormatDelta formats the difference between current and previous divergence values.
func FormatDelta(current, previous float64) string {
	delta := current - previous
	if delta >= 0 {
		return fmt.Sprintf("+%f", delta)
	}
	return fmt.Sprintf("%f", delta)
}

// MeterResult holds the complete output of Phase 2 scoring orchestration.
type MeterResult struct {
	Divergence      DivergenceResult
	DMailCandidates []ClaudeDMailCandidate
	Reasoning       string
	ImpactRadius    []ImpactEntry
}

// DivergenceMeter bridges Claude output and the scoring engine.
type DivergenceMeter struct {
	Config Config
}

// ProcessResponse takes a ClaudeResponse, runs CalcDivergence and
// DetermineSeverity, and returns a MeterResult.
func (dm *DivergenceMeter) ProcessResponse(resp ClaudeResponse) MeterResult {
	divergence := CalcDivergence(resp.Axes, dm.Config.Weights)
	severityCfg := SeverityConfig{
		Thresholds:      dm.Config.Thresholds,
		PerAxisOverride: dm.Config.PerAxisOverride,
	}
	divergence = DetermineSeverity(divergence, severityCfg)
	return MeterResult{
		Divergence:      divergence,
		DMailCandidates: resp.DMails,
		Reasoning:       resp.Reasoning,
		ImpactRadius:    resp.ImpactRadius,
	}
}
