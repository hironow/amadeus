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
	Value        float64            `json:"divergence"`
	Internal     float64            `json:"internal"`
	Axes         map[Axis]AxisScore `json:"axes"`
	Severity     Severity           `json:"severity"`
	Overridden   bool               `json:"overridden"`
	MissingAxes  []Axis             `json:"missing_axes,omitempty"`
	ADRAlignment ADRAlignmentMap    `json:"adr_alignment,omitempty"` // E19: per-ADR scores
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
	ADRForceHigh        int      `yaml:"adr_integrity_force_high" json:"adr_integrity_force_high"`
	DoDForceHigh        int      `yaml:"dod_fulfillment_force_high" json:"dod_fulfillment_force_high"`
	DepForceMedium      int      `yaml:"dependency_integrity_force_medium" json:"dependency_integrity_force_medium"`
	ImplicitForceMedium int      `yaml:"implicit_constraints_force_medium" json:"implicit_constraints_force_medium"`
	ADRCritical         []string `yaml:"adr_critical,omitempty" json:"adr_critical,omitempty"` // E19: ADR numbers that force HIGH when violated
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
			ADRForceHigh:        60,
			DoDForceHigh:        70,
			DepForceMedium:      80,
			ImplicitForceMedium: 80,
		},
	}
}

// RequiredAxes lists the axes that must be present for a valid divergence calculation.
var RequiredAxes = []Axis{AxisADR, AxisDoD, AxisDependency, AxisImplicit}

// ValidateAxesPresent checks that all required axes are present in the map.
// Returns a list of missing axis names, or nil if all are present.
func ValidateAxesPresent(axes map[Axis]AxisScore) []Axis {
	var missing []Axis
	for _, axis := range RequiredAxes {
		if _, ok := axes[axis]; !ok {
			missing = append(missing, axis)
		}
	}
	return missing
}

// ClampAxisScore clamps a score to the valid range [0, 100].
func ClampAxisScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

// ClampAxesMap returns a new axes map with all scores clamped to [0, 100].
func ClampAxesMap(axes map[Axis]AxisScore) map[Axis]AxisScore {
	clamped := make(map[Axis]AxisScore, len(axes))
	for axis, as := range axes {
		clamped[axis] = AxisScore{
			Score:   ClampAxisScore(as.Score),
			Details: as.Details,
		}
	}
	return clamped
}

// CalcDivergence computes the weighted divergence score from axis scores.
// If required axes are missing, it returns a result with MissingAxes populated.
func CalcDivergence(axes map[Axis]AxisScore, weights Weights) DivergenceResult {
	internal := float64(axes[AxisADR].Score)*weights.ADRIntegrity +
		float64(axes[AxisDoD].Score)*weights.DoDFulfillment +
		float64(axes[AxisDependency].Score)*weights.DependencyIntegrity +
		float64(axes[AxisImplicit].Score)*weights.ImplicitConstraints

	result := DivergenceResult{
		Value:    internal / 100.0,
		Internal: internal,
		Axes:     axes,
	}
	result.MissingAxes = ValidateAxesPresent(axes)
	return result
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
	if config.PerAxisOverride.ImplicitForceMedium > 0 && result.Axes[AxisImplicit].Score >= config.PerAxisOverride.ImplicitForceMedium {
		if severity == SeverityLow {
			overridden = true
			severity = SeverityMedium
		}
	}

	// E19: Per-ADR critical override — specific ADRs force HIGH when violated
	for _, criticalNum := range config.PerAxisOverride.ADRCritical {
		if a, ok := result.ADRAlignment[criticalNum]; ok && a.Verdict == "violated" {
			if severity != SeverityHigh {
				overridden = true
			}
			severity = SeverityHigh
			break
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

// WeightForAxis returns the configured weight for a given axis.
func WeightForAxis(axis Axis, w Weights) float64 {
	switch axis {
	case AxisADR:
		return w.ADRIntegrity
	case AxisDoD:
		return w.DoDFulfillment
	case AxisDependency:
		return w.DependencyIntegrity
	case AxisImplicit:
		return w.ImplicitConstraints
	default:
		return 0
	}
}

// ClassifyByAxes performs rule-based classification of feedback into "design"
// or "implementation" category based on axis scores and weights.
// Design axes: ADR integrity, dependency integrity.
// Implementation axes: DoD fulfillment, implicit constraints.
// On tie, defaults to "design".
func ClassifyByAxes(axes map[Axis]AxisScore, weights Weights) string {
	designScore := float64(axes[AxisADR].Score)*weights.ADRIntegrity +
		float64(axes[AxisDependency].Score)*weights.DependencyIntegrity
	implScore := float64(axes[AxisDoD].Score)*weights.DoDFulfillment +
		float64(axes[AxisImplicit].Score)*weights.ImplicitConstraints
	if designScore >= implScore {
		return "design"
	}
	return "implementation"
}

// ResolveFeedbackKinds determines the D-Mail kind(s) from qualitative (LLM)
// and quantitative (rule-based) classification signals.
// When both agree, returns a single kind. On disagreement, returns both.
func ResolveFeedbackKinds(qualitative, quantitative string) []DMailKind {
	if qualitative == quantitative {
		if qualitative == "design" {
			return []DMailKind{KindDesignFeedback}
		}
		return []DMailKind{KindImplFeedback}
	}
	return []DMailKind{KindDesignFeedback, KindImplFeedback}
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
	// #089: Clamp axes to [0,100] before scoring to handle out-of-range LLM outputs.
	clamped := ClampAxesMap(resp.Axes)

	// #123: Validate all required axes are present and record any missing.
	missing := ValidateAxesPresent(clamped)

	divergence := CalcDivergence(clamped, dm.Config.Weights)
	// #124: Populate MissingAxes from validation result.
	divergence.MissingAxes = missing

	// E19: Attach per-ADR alignment and override adr_integrity axis when available.
	if len(resp.ADRAlignment) > 0 {
		divergence.ADRAlignment = resp.ADRAlignment
		derivedADR := DeriveADRIntegrityScore(resp.ADRAlignment)
		if axis, ok := divergence.Axes[AxisADR]; ok {
			axis.Score = derivedADR
			divergence.Axes[AxisADR] = axis
		}
	}

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
