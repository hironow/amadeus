package amadeus

import (
	"fmt"
	"math"
)

// ValidLang reports whether lang is a supported language code.
func ValidLang(lang string) bool {
	return lang == "ja" || lang == "en"
}

// ConvergenceConfig controls the world-line convergence detection parameters.
type ConvergenceConfig struct {
	WindowDays           int `yaml:"window_days"`
	Threshold            int `yaml:"threshold"`
	EscalationMultiplier int `yaml:"escalation_multiplier"`
}

// Config holds the complete Amadeus configuration.
type Config struct {
	Lang            string            `yaml:"lang"`
	Weights         Weights           `yaml:"weights"`
	Thresholds      Thresholds        `yaml:"thresholds"`
	PerAxisOverride PerAxisOverride   `yaml:"per_axis_override"`
	FullCheck       FullCheckConfig   `yaml:"full_check"`
	Convergence     ConvergenceConfig `yaml:"convergence"`
}

// FullCheckConfig controls the full scan strategy.
type FullCheckConfig struct {
	Interval         int     `yaml:"interval"`
	OnDivergenceJump float64 `yaml:"on_divergence_jump"`
}

// DefaultConfig returns a Config populated with architecture-document defaults.
func DefaultConfig() Config {
	sc := DefaultThresholds()
	return Config{
		Lang:            "ja",
		Weights:         DefaultWeights(),
		Thresholds:      sc.Thresholds,
		PerAxisOverride: sc.PerAxisOverride,
		FullCheck: FullCheckConfig{
			Interval:         10,
			OnDivergenceJump: 0.15,
		},
		Convergence: ConvergenceConfig{
			WindowDays:           14,
			Threshold:            3,
			EscalationMultiplier: 2,
		},
	}
}

// ValidateConfig checks the config for consistency and returns a list of errors.
// An empty slice means the config is valid.
func ValidateConfig(cfg Config) []string {
	var errs []string

	// Language check
	if !ValidLang(cfg.Lang) {
		errs = append(errs, fmt.Sprintf("lang must be \"ja\" or \"en\" (got %q)", cfg.Lang))
	}

	// Weight range checks
	weights := []struct {
		name  string
		value float64
	}{
		{"adr_integrity", cfg.Weights.ADRIntegrity},
		{"dod_fulfillment", cfg.Weights.DoDFulfillment},
		{"dependency_integrity", cfg.Weights.DependencyIntegrity},
		{"implicit_constraints", cfg.Weights.ImplicitConstraints},
	}
	for _, w := range weights {
		if w.value < 0 || w.value > 1 {
			errs = append(errs, fmt.Sprintf("weights.%s must be between 0.0 and 1.0 (got %f)", w.name, w.value))
		}
	}

	// Weights sum check
	sum := cfg.Weights.ADRIntegrity + cfg.Weights.DoDFulfillment +
		cfg.Weights.DependencyIntegrity + cfg.Weights.ImplicitConstraints
	if math.Abs(sum-1.0) > 0.001 {
		errs = append(errs, fmt.Sprintf("weights must sum to 1.0 (got %f)", sum))
	}

	// Threshold order check
	if cfg.Thresholds.LowMax >= cfg.Thresholds.MediumMax {
		errs = append(errs, fmt.Sprintf("thresholds: low_max (%f) must be less than medium_max (%f)",
			cfg.Thresholds.LowMax, cfg.Thresholds.MediumMax))
	}

	// Per-axis override range checks
	overrides := []struct {
		name  string
		value int
	}{
		{"adr_integrity_force_high", cfg.PerAxisOverride.ADRForceHigh},
		{"dod_fulfillment_force_high", cfg.PerAxisOverride.DoDForceHigh},
		{"dependency_integrity_force_medium", cfg.PerAxisOverride.DepForceMedium},
	}
	for _, o := range overrides {
		if o.value < 0 || o.value > 100 {
			errs = append(errs, fmt.Sprintf("per_axis_override.%s must be between 0 and 100 (got %d)", o.name, o.value))
		}
	}

	// Convergence config
	if cfg.Convergence.WindowDays <= 0 {
		errs = append(errs, fmt.Sprintf("convergence.window_days must be positive (got %d)", cfg.Convergence.WindowDays))
	}
	if cfg.Convergence.Threshold <= 0 {
		errs = append(errs, fmt.Sprintf("convergence.threshold must be positive (got %d)", cfg.Convergence.Threshold))
	}
	if cfg.Convergence.EscalationMultiplier < 0 {
		errs = append(errs, fmt.Sprintf("convergence.escalation_multiplier must be non-negative (got %d)", cfg.Convergence.EscalationMultiplier))
	}

	// Full check config
	if cfg.FullCheck.Interval <= 0 {
		errs = append(errs, fmt.Sprintf("full_check.interval must be positive (got %d)", cfg.FullCheck.Interval))
	}
	if cfg.FullCheck.OnDivergenceJump < 0 {
		errs = append(errs, fmt.Sprintf("full_check.on_divergence_jump must be non-negative (got %f)", cfg.FullCheck.OnDivergenceJump))
	}

	return errs
}
