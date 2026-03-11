package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	cfg := domain.DefaultConfig()
	if cfg.Weights.ADRIntegrity != 0.4 {
		t.Errorf("expected ADR weight 0.4, got %f", cfg.Weights.ADRIntegrity)
	}
	if cfg.Weights.DoDFulfillment != 0.3 {
		t.Errorf("expected DoD weight 0.3, got %f", cfg.Weights.DoDFulfillment)
	}
	if cfg.FullCheck.Interval != 10 {
		t.Errorf("expected interval 10, got %d", cfg.FullCheck.Interval)
	}
}

func TestDefaultConfig_AllFields(t *testing.T) {
	// given/when
	cfg := domain.DefaultConfig()

	// then: Lang
	if cfg.Lang != "ja" {
		t.Errorf("Lang: expected 'ja', got %q", cfg.Lang)
	}

	// then: Weights
	if cfg.Weights.ADRIntegrity != 0.4 {
		t.Errorf("Weights.ADRIntegrity: expected 0.4, got %f", cfg.Weights.ADRIntegrity)
	}
	if cfg.Weights.DoDFulfillment != 0.3 {
		t.Errorf("Weights.DoDFulfillment: expected 0.3, got %f", cfg.Weights.DoDFulfillment)
	}
	if cfg.Weights.DependencyIntegrity != 0.2 {
		t.Errorf("Weights.DependencyIntegrity: expected 0.2, got %f", cfg.Weights.DependencyIntegrity)
	}
	if cfg.Weights.ImplicitConstraints != 0.1 {
		t.Errorf("Weights.ImplicitConstraints: expected 0.1, got %f", cfg.Weights.ImplicitConstraints)
	}

	// then: Thresholds
	if cfg.Thresholds.LowMax != 0.25 {
		t.Errorf("Thresholds.LowMax: expected 0.25, got %f", cfg.Thresholds.LowMax)
	}
	if cfg.Thresholds.MediumMax != 0.50 {
		t.Errorf("Thresholds.MediumMax: expected 0.50, got %f", cfg.Thresholds.MediumMax)
	}

	// then: PerAxisOverride
	if cfg.PerAxisOverride.ADRForceHigh != 60 {
		t.Errorf("PerAxisOverride.ADRForceHigh: expected 60, got %d", cfg.PerAxisOverride.ADRForceHigh)
	}
	if cfg.PerAxisOverride.DoDForceHigh != 70 {
		t.Errorf("PerAxisOverride.DoDForceHigh: expected 70, got %d", cfg.PerAxisOverride.DoDForceHigh)
	}
	if cfg.PerAxisOverride.DepForceMedium != 80 {
		t.Errorf("PerAxisOverride.DepForceMedium: expected 80, got %d", cfg.PerAxisOverride.DepForceMedium)
	}

	// then: FullCheck
	if cfg.FullCheck.Interval != 10 {
		t.Errorf("FullCheck.Interval: expected 10, got %d", cfg.FullCheck.Interval)
	}
	if cfg.FullCheck.OnDivergenceJump != 0.15 {
		t.Errorf("FullCheck.OnDivergenceJump: expected 0.15, got %f", cfg.FullCheck.OnDivergenceJump)
	}

	// then: Convergence
	if cfg.Convergence.WindowDays != 14 {
		t.Errorf("Convergence.WindowDays: expected 14, got %d", cfg.Convergence.WindowDays)
	}
	if cfg.Convergence.Threshold != 3 {
		t.Errorf("Convergence.Threshold: expected 3, got %d", cfg.Convergence.Threshold)
	}
	if cfg.Convergence.EscalationMultiplier != 2 {
		t.Errorf("Convergence.EscalationMultiplier: expected 2, got %d", cfg.Convergence.EscalationMultiplier)
	}
}

func TestValidateConfig_DefaultIsValid(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()

	// when
	errs := domain.ValidateConfig(cfg)

	// then
	if len(errs) != 0 {
		t.Errorf("default config should be valid, got errors: %v", errs)
	}
}

func TestValidateConfig_WeightsSumNot1(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()
	cfg.Weights.ADRIntegrity = 0.5 // sum = 0.5+0.3+0.2+0.1 = 1.1

	// when
	errs := domain.ValidateConfig(cfg)

	// then
	if len(errs) == 0 {
		t.Fatal("expected error for weights sum != 1.0")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "sum") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'sum' in error, got: %v", errs)
	}
}

func TestValidateConfig_NegativeWeight(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()
	cfg.Weights.ADRIntegrity = -0.1

	// when
	errs := domain.ValidateConfig(cfg)

	// then
	if len(errs) == 0 {
		t.Fatal("expected error for negative weight")
	}
}

func TestValidateConfig_ThresholdsOutOfOrder(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()
	cfg.Thresholds.LowMax = 0.6
	cfg.Thresholds.MediumMax = 0.3

	// when
	errs := domain.ValidateConfig(cfg)

	// then
	if len(errs) == 0 {
		t.Fatal("expected error for out-of-order thresholds")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "low_max") && strings.Contains(e, "medium_max") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected threshold order error, got: %v", errs)
	}
}

func TestValidateConfig_PerAxisOverrideOutOfRange(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()
	cfg.PerAxisOverride.ADRForceHigh = 150

	// when
	errs := domain.ValidateConfig(cfg)

	// then
	if len(errs) == 0 {
		t.Fatal("expected error for override > 100")
	}
}

func TestValidateConfig_FullCheckIntervalZero(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()
	cfg.FullCheck.Interval = 0

	// when
	errs := domain.ValidateConfig(cfg)

	// then
	if len(errs) == 0 {
		t.Fatal("expected error for zero interval")
	}
}

func TestValidateConfig_DivergenceJumpNegative(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()
	cfg.FullCheck.OnDivergenceJump = -0.1

	// when
	errs := domain.ValidateConfig(cfg)

	// then
	if len(errs) == 0 {
		t.Fatal("expected error for negative divergence jump")
	}
}

func TestValidateConfig_MultipleErrors(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()
	cfg.Weights.ADRIntegrity = -0.1
	cfg.Thresholds.LowMax = 0.8
	cfg.FullCheck.Interval = 0

	// when
	errs := domain.ValidateConfig(cfg)

	// then
	if len(errs) < 3 {
		t.Errorf("expected at least 3 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidLang_Ja(t *testing.T) {
	// when
	result := domain.ValidLang("ja")

	// then
	if !result {
		t.Error("expected ValidLang(\"ja\") to be true")
	}
}

func TestValidLang_En(t *testing.T) {
	// when
	result := domain.ValidLang("en")

	// then
	if !result {
		t.Error("expected ValidLang(\"en\") to be true")
	}
}

func TestValidLang_Unknown(t *testing.T) {
	// when
	result := domain.ValidLang("fr")

	// then
	if result {
		t.Error("expected ValidLang(\"fr\") to be false")
	}
}

func TestValidLang_Empty(t *testing.T) {
	// when
	result := domain.ValidLang("")

	// then
	if result {
		t.Error("expected ValidLang(\"\") to be false")
	}
}

func TestDefaultConfig_LangIsJa(t *testing.T) {
	// when
	cfg := domain.DefaultConfig()

	// then
	if cfg.Lang != "ja" {
		t.Errorf("expected default Lang=\"ja\", got %q", cfg.Lang)
	}
}

func TestValidateConfig_ConvergenceWindowDaysZero(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()
	cfg.Convergence.WindowDays = 0

	// when
	errs := domain.ValidateConfig(cfg)

	// then
	found := false
	for _, e := range errs {
		if strings.Contains(e, "convergence.window_days") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected validation error mentioning 'convergence.window_days', got: %v", errs)
	}
}

func TestValidateConfig_ConvergenceThresholdZero(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()
	cfg.Convergence.Threshold = 0

	// when
	errs := domain.ValidateConfig(cfg)

	// then
	found := false
	for _, e := range errs {
		if strings.Contains(e, "convergence.threshold") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected validation error mentioning 'convergence.threshold', got: %v", errs)
	}
}

func TestConfig_ComputedConfig_EmptyByDefault(t *testing.T) {
	// when
	cfg := domain.DefaultConfig()

	// then
	if cfg.Computed != (domain.ComputedConfig{}) {
		t.Errorf("expected Computed to be zero-value, got %+v", cfg.Computed)
	}
}

func TestDefaultConfig_ClaudeCmd(t *testing.T) {
	// when
	cfg := domain.DefaultConfig()

	// then
	if cfg.ClaudeCmd != "claude" {
		t.Errorf("expected ClaudeCmd=\"claude\", got %q", cfg.ClaudeCmd)
	}
	if cfg.Model != "opus" {
		t.Errorf("expected Model=\"opus\", got %q", cfg.Model)
	}
	if cfg.TimeoutSec != 1980 {
		t.Errorf("expected TimeoutSec=1980, got %d", cfg.TimeoutSec)
	}
}

func TestConfig_YAMLRoundTrip_NoComputedKey(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()

	// when: marshal
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	// then: no "computed" key in YAML output (empty struct with omitempty)
	if strings.Contains(string(data), "computed") {
		t.Errorf("expected no 'computed' key in YAML, got:\n%s", string(data))
	}

	// when: unmarshal back
	var restored domain.Config
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	// then: ClaudeCmd preserved
	if restored.ClaudeCmd != "claude" {
		t.Errorf("expected ClaudeCmd=\"claude\" after round-trip, got %q", restored.ClaudeCmd)
	}
	// then: Model preserved
	if restored.Model != "opus" {
		t.Errorf("expected Model=\"opus\" after round-trip, got %q", restored.Model)
	}
	// then: TimeoutSec preserved
	if restored.TimeoutSec != 1980 {
		t.Errorf("expected TimeoutSec=1980 after round-trip, got %d", restored.TimeoutSec)
	}
}

func TestDefaultConfig_WaitTimeout(t *testing.T) {
	// when
	cfg := domain.DefaultConfig()

	// then
	if cfg.WaitTimeout != domain.DefaultWaitTimeout {
		t.Errorf("expected WaitTimeout=%v, got %v", domain.DefaultWaitTimeout, cfg.WaitTimeout)
	}
}

func TestDefaultWaitTimeout_Is30Minutes(t *testing.T) {
	// then
	if domain.DefaultWaitTimeout != 30*time.Minute {
		t.Errorf("expected 30m, got %v", domain.DefaultWaitTimeout)
	}
}

func TestConfig_NegativeWaitTimeout_DisablesWaiting(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()
	cfg.WaitTimeout = -1

	// then: negative WaitTimeout disables waiting mode (no validation error)
	errs := domain.ValidateConfig(cfg)
	for _, e := range errs {
		if strings.Contains(e, "wait_timeout") {
			t.Errorf("negative WaitTimeout should be valid (disables waiting), got error: %s", e)
		}
	}
}

func TestValidateConfig_InvalidLang(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()
	cfg.Lang = "fr"

	// when
	errs := domain.ValidateConfig(cfg)

	// then
	found := false
	for _, e := range errs {
		if strings.Contains(e, "lang") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected validation error mentioning 'lang', got: %v", errs)
	}
}
