package amadeus_test

import (
	"strings"
	"testing"

	"github.com/hironow/amadeus"
)

func TestDefaultConfig(t *testing.T) {
	cfg := amadeus.DefaultConfig()
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

func TestValidateConfig_DefaultIsValid(t *testing.T) {
	// given
	cfg := amadeus.DefaultConfig()

	// when
	errs := amadeus.ValidateConfig(cfg)

	// then
	if len(errs) != 0 {
		t.Errorf("default config should be valid, got errors: %v", errs)
	}
}

func TestValidateConfig_WeightsSumNot1(t *testing.T) {
	// given
	cfg := amadeus.DefaultConfig()
	cfg.Weights.ADRIntegrity = 0.5 // sum = 0.5+0.3+0.2+0.1 = 1.1

	// when
	errs := amadeus.ValidateConfig(cfg)

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
	cfg := amadeus.DefaultConfig()
	cfg.Weights.ADRIntegrity = -0.1

	// when
	errs := amadeus.ValidateConfig(cfg)

	// then
	if len(errs) == 0 {
		t.Fatal("expected error for negative weight")
	}
}

func TestValidateConfig_ThresholdsOutOfOrder(t *testing.T) {
	// given
	cfg := amadeus.DefaultConfig()
	cfg.Thresholds.LowMax = 0.6
	cfg.Thresholds.MediumMax = 0.3

	// when
	errs := amadeus.ValidateConfig(cfg)

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
	cfg := amadeus.DefaultConfig()
	cfg.PerAxisOverride.ADRForceHigh = 150

	// when
	errs := amadeus.ValidateConfig(cfg)

	// then
	if len(errs) == 0 {
		t.Fatal("expected error for override > 100")
	}
}

func TestValidateConfig_FullCheckIntervalZero(t *testing.T) {
	// given
	cfg := amadeus.DefaultConfig()
	cfg.FullCheck.Interval = 0

	// when
	errs := amadeus.ValidateConfig(cfg)

	// then
	if len(errs) == 0 {
		t.Fatal("expected error for zero interval")
	}
}

func TestValidateConfig_DivergenceJumpNegative(t *testing.T) {
	// given
	cfg := amadeus.DefaultConfig()
	cfg.FullCheck.OnDivergenceJump = -0.1

	// when
	errs := amadeus.ValidateConfig(cfg)

	// then
	if len(errs) == 0 {
		t.Fatal("expected error for negative divergence jump")
	}
}

func TestValidateConfig_MultipleErrors(t *testing.T) {
	// given
	cfg := amadeus.DefaultConfig()
	cfg.Weights.ADRIntegrity = -0.1
	cfg.Thresholds.LowMax = 0.8
	cfg.FullCheck.Interval = 0

	// when
	errs := amadeus.ValidateConfig(cfg)

	// then
	if len(errs) < 3 {
		t.Errorf("expected at least 3 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidLang_Ja(t *testing.T) {
	// when
	result := amadeus.ValidLang("ja")

	// then
	if !result {
		t.Error("expected ValidLang(\"ja\") to be true")
	}
}

func TestValidLang_En(t *testing.T) {
	// when
	result := amadeus.ValidLang("en")

	// then
	if !result {
		t.Error("expected ValidLang(\"en\") to be true")
	}
}

func TestValidLang_Unknown(t *testing.T) {
	// when
	result := amadeus.ValidLang("fr")

	// then
	if result {
		t.Error("expected ValidLang(\"fr\") to be false")
	}
}

func TestValidLang_Empty(t *testing.T) {
	// when
	result := amadeus.ValidLang("")

	// then
	if result {
		t.Error("expected ValidLang(\"\") to be false")
	}
}

func TestDefaultConfig_LangIsJa(t *testing.T) {
	// when
	cfg := amadeus.DefaultConfig()

	// then
	if cfg.Lang != "ja" {
		t.Errorf("expected default Lang=\"ja\", got %q", cfg.Lang)
	}
}

func TestValidateConfig_ConvergenceWindowDaysZero(t *testing.T) {
	// given
	cfg := amadeus.DefaultConfig()
	cfg.Convergence.WindowDays = 0

	// when
	errs := amadeus.ValidateConfig(cfg)

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
	cfg := amadeus.DefaultConfig()
	cfg.Convergence.Threshold = 0

	// when
	errs := amadeus.ValidateConfig(cfg)

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

func TestValidateConfig_InvalidLang(t *testing.T) {
	// given
	cfg := amadeus.DefaultConfig()
	cfg.Lang = "fr"

	// when
	errs := amadeus.ValidateConfig(cfg)

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
