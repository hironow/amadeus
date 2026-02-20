package amadeus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
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

func TestLoadConfig_FromFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := `weights:
  adr_integrity: 0.5
  dod_fulfillment: 0.25
  dependency_integrity: 0.15
  implicit_constraints: 0.1

thresholds:
  low_max: 0.200000
  medium_max: 0.400000

full_check:
  interval: 5
  on_divergence_jump: 0.20
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Weights.ADRIntegrity != 0.5 {
		t.Errorf("expected ADR weight 0.5, got %f", cfg.Weights.ADRIntegrity)
	}
	if cfg.Thresholds.LowMax != 0.2 {
		t.Errorf("expected low_max 0.2, got %f", cfg.Thresholds.LowMax)
	}
	if cfg.FullCheck.Interval != 5 {
		t.Errorf("expected interval 5, got %d", cfg.FullCheck.Interval)
	}
}

func TestLoadConfig_FileNotFound_ReturnsDefault(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg.Weights.ADRIntegrity != 0.4 {
		t.Errorf("expected default ADR weight 0.4, got %f", cfg.Weights.ADRIntegrity)
	}
}

func TestValidateConfig_DefaultIsValid(t *testing.T) {
	// given
	cfg := DefaultConfig()

	// when
	errs := ValidateConfig(cfg)

	// then
	if len(errs) != 0 {
		t.Errorf("default config should be valid, got errors: %v", errs)
	}
}

func TestValidateConfig_WeightsSumNot1(t *testing.T) {
	// given
	cfg := DefaultConfig()
	cfg.Weights.ADRIntegrity = 0.5 // sum = 0.5+0.3+0.2+0.1 = 1.1

	// when
	errs := ValidateConfig(cfg)

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
	cfg := DefaultConfig()
	cfg.Weights.ADRIntegrity = -0.1

	// when
	errs := ValidateConfig(cfg)

	// then
	if len(errs) == 0 {
		t.Fatal("expected error for negative weight")
	}
}

func TestValidateConfig_ThresholdsOutOfOrder(t *testing.T) {
	// given
	cfg := DefaultConfig()
	cfg.Thresholds.LowMax = 0.6
	cfg.Thresholds.MediumMax = 0.3

	// when
	errs := ValidateConfig(cfg)

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
	cfg := DefaultConfig()
	cfg.PerAxisOverride.ADRForceHigh = 150

	// when
	errs := ValidateConfig(cfg)

	// then
	if len(errs) == 0 {
		t.Fatal("expected error for override > 100")
	}
}

func TestValidateConfig_FullCheckIntervalZero(t *testing.T) {
	// given
	cfg := DefaultConfig()
	cfg.FullCheck.Interval = 0

	// when
	errs := ValidateConfig(cfg)

	// then
	if len(errs) == 0 {
		t.Fatal("expected error for zero interval")
	}
}

func TestValidateConfig_DivergenceJumpNegative(t *testing.T) {
	// given
	cfg := DefaultConfig()
	cfg.FullCheck.OnDivergenceJump = -0.1

	// when
	errs := ValidateConfig(cfg)

	// then
	if len(errs) == 0 {
		t.Fatal("expected error for negative divergence jump")
	}
}

func TestValidateConfig_MultipleErrors(t *testing.T) {
	// given
	cfg := DefaultConfig()
	cfg.Weights.ADRIntegrity = -0.1
	cfg.Thresholds.LowMax = 0.8
	cfg.FullCheck.Interval = 0

	// when
	errs := ValidateConfig(cfg)

	// then
	if len(errs) < 3 {
		t.Errorf("expected at least 3 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidLang_Ja(t *testing.T) {
	// when
	result := ValidLang("ja")

	// then
	if !result {
		t.Error("expected ValidLang(\"ja\") to be true")
	}
}

func TestValidLang_En(t *testing.T) {
	// when
	result := ValidLang("en")

	// then
	if !result {
		t.Error("expected ValidLang(\"en\") to be true")
	}
}

func TestValidLang_Unknown(t *testing.T) {
	// when
	result := ValidLang("fr")

	// then
	if result {
		t.Error("expected ValidLang(\"fr\") to be false")
	}
}

func TestValidLang_Empty(t *testing.T) {
	// when
	result := ValidLang("")

	// then
	if result {
		t.Error("expected ValidLang(\"\") to be false")
	}
}

func TestDefaultConfig_LangIsJa(t *testing.T) {
	// when
	cfg := DefaultConfig()

	// then
	if cfg.Lang != "ja" {
		t.Errorf("expected default Lang=\"ja\", got %q", cfg.Lang)
	}
}

func TestValidateConfig_InvalidLang(t *testing.T) {
	// given
	cfg := DefaultConfig()
	cfg.Lang = "fr"

	// when
	errs := ValidateConfig(cfg)

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
