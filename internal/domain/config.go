package domain

import (
	"fmt"
	"math"
	"time"
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

// ComputedConfig holds system-written fields. Empty for amadeus today.
type ComputedConfig struct{}

type ImprovementCollectorConfig struct {
	Enabled       *bool    `yaml:"enabled,omitempty"`
	ProjectID     string   `yaml:"project_id,omitempty"`
	APIURL        string   `yaml:"api_url,omitempty"`
	QueryLimit    int      `yaml:"query_limit,omitempty"`
	FeedbackTypes []string `yaml:"feedback_types,omitempty"`
}

// Default values for Config fields. Used by DefaultConfig and post-load
// validation to avoid hardcoded strings throughout the codebase.
const (
	DefaultClaudeCmd  = "claude"
	DefaultModel      = "opus"
	DefaultTimeoutSec = 1980
)

// DefaultIdleTimeout is the default D-Mail waiting phase timeout.
const DefaultIdleTimeout = 30 * time.Minute

// ApproverConfig describes how approval behavior is configured.
// Implemented by FlagApproverConfig. Used by session.BuildApprover.
type ApproverConfig interface {
	IsAutoApprove() bool
	ApproveCmdString() string
}

// FlagApproverConfig adapts CLI flag values to the ApproverConfig interface.
type FlagApproverConfig struct {
	AutoApprove bool
	ApproveCmd  string
}

// IsAutoApprove reports whether auto-approve is enabled.
func (f FlagApproverConfig) IsAutoApprove() bool { return f.AutoApprove }

// ApproveCmdString returns the approval command string.
func (f FlagApproverConfig) ApproveCmdString() string { return f.ApproveCmd }

// Config holds the complete Amadeus configuration.
type Config struct {
	Lang                 string                     `yaml:"lang"`
	ClaudeCmd            string                     `yaml:"claude_cmd,omitempty"`
	Model                string                     `yaml:"model,omitempty"`
	TimeoutSec           int                        `yaml:"timeout_sec,omitempty"`
	Weights              Weights                    `yaml:"weights"`
	Thresholds           Thresholds                 `yaml:"thresholds"`
	PerAxisOverride      PerAxisOverride            `yaml:"per_axis_override"`
	FullCheck            FullCheckConfig            `yaml:"full_check"`
	Convergence          ConvergenceConfig          `yaml:"convergence"`
	BaselineStaleness    BaselineStalenessConfig    `yaml:"baseline_staleness,omitempty"`
	ImprovementCollector ImprovementCollectorConfig `yaml:"improvement_collector,omitempty"`
	IdleTimeout          time.Duration              `yaml:"idle_timeout,omitempty"`
	Computed             ComputedConfig             `yaml:"computed,omitempty"`
}

// DefaultMaxResultHistory is the default maximum number of check results to
// retain during event replay / projection rebuild.
const DefaultMaxResultHistory = 100

// FullCheckConfig controls the full scan strategy.
type FullCheckConfig struct {
	Interval         int     `yaml:"interval"`
	OnDivergenceJump float64 `yaml:"on_divergence_jump"`
	MaxResultHistory int     `yaml:"max_result_history,omitempty"`
}

// BaselineStalenessConfig controls auto-promotion to full calibration when the
// last check is older than MaxAgeDays. Disabled when MaxAgeDays is 0.
type BaselineStalenessConfig struct {
	MaxAgeDays int `yaml:"max_age_days"`
}

// IsStale reports whether the given checkedAt timestamp is older than MaxAgeDays.
// Returns false when MaxAgeDays is 0 (disabled) or checkedAt is the zero value.
func (b BaselineStalenessConfig) IsStale(checkedAt time.Time) bool {
	if b.MaxAgeDays == 0 {
		return false
	}
	if checkedAt.IsZero() {
		return false
	}
	return time.Since(checkedAt) > time.Duration(b.MaxAgeDays)*24*time.Hour
}

// DefaultConfig returns a Config populated with architecture-document defaults.
func DefaultConfig() Config {
	sc := DefaultThresholds()
	return Config{
		Lang:            "ja",
		ClaudeCmd:       DefaultClaudeCmd,
		Model:           DefaultModel,
		TimeoutSec:      DefaultTimeoutSec,
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
		IdleTimeout: DefaultIdleTimeout,
	}
}

// ConfigLang returns the configured language code.
func (c Config) ConfigLang() string { return c.Lang }

// WeightFor returns the configured weight for a given axis.
func (c Config) WeightFor(axis Axis) float64 { return WeightForAxis(axis, c.Weights) }

// DetectConvergence analyzes D-Mails for recurring patterns using the config's convergence settings.
func (c Config) DetectConvergence(dmails []DMail, now time.Time) []ConvergenceAlert {
	return AnalyzeConvergence(dmails, c.Convergence, now)
}

// ValidateConfig checks the config for consistency and returns a list of errors.
// An empty slice means the config is valid.
func ValidateConfig(cfg Config) []string {
	var errs []string

	// Required string fields
	if cfg.ClaudeCmd == "" {
		errs = append(errs, "claude_cmd must not be empty")
	}
	if cfg.Model == "" {
		errs = append(errs, "model must not be empty")
	}

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
		{"implicit_constraints_force_medium", cfg.PerAxisOverride.ImplicitForceMedium},
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

	// TimeoutSec check
	if cfg.TimeoutSec < 0 {
		errs = append(errs, fmt.Sprintf("timeout_sec must be non-negative (got %d)", cfg.TimeoutSec))
	}
	if cfg.ImprovementCollector.QueryLimit < 0 {
		errs = append(errs, fmt.Sprintf("improvement_collector.query_limit must be non-negative (got %d)", cfg.ImprovementCollector.QueryLimit))
	}

	// Full check config
	if cfg.FullCheck.Interval <= 0 {
		errs = append(errs, fmt.Sprintf("full_check.interval must be positive (got %d)", cfg.FullCheck.Interval))
	}
	if cfg.FullCheck.OnDivergenceJump < 0 {
		errs = append(errs, fmt.Sprintf("full_check.on_divergence_jump must be non-negative (got %f)", cfg.FullCheck.OnDivergenceJump))
	}
	if cfg.FullCheck.MaxResultHistory < 0 {
		errs = append(errs, fmt.Sprintf("full_check.max_result_history must be non-negative (got %d)", cfg.FullCheck.MaxResultHistory))
	}

	// Cross-field semantic constraints

	// Check #1: OnDivergenceJump >= MediumMax skips medium severity entirely
	if cfg.FullCheck.OnDivergenceJump >= cfg.Thresholds.MediumMax {
		errs = append(errs, fmt.Sprintf(
			"full_check.on_divergence_jump (%f) must be less than thresholds.medium_max (%f)",
			cfg.FullCheck.OnDivergenceJump, cfg.Thresholds.MediumMax))
	}

	// Check #3: IdleTimeout must not exceed WindowDays * 24h (when IdleTimeout is positive)
	if cfg.IdleTimeout > 0 && cfg.IdleTimeout > time.Duration(cfg.Convergence.WindowDays)*24*time.Hour {
		errs = append(errs, fmt.Sprintf(
			"idle_timeout (%v) must not exceed convergence.window_days (%d) * 24h (%v)",
			cfg.IdleTimeout, cfg.Convergence.WindowDays,
			time.Duration(cfg.Convergence.WindowDays)*24*time.Hour))
	}

	return errs
}

// RunOptions configures the amadeus run daemon loop.
type RunOptions struct {
	CheckOptions        // embedded check options
	BaseBranch   string // upstream branch for post-merge checks (empty = none)
	AutoMerge    bool   // auto-merge eligible PRs when no drift detected (default: true when BaseBranch is set)
	ReadyLabel   string // issue label that signals "ready to close" (default: "sightjack:ready")
}
