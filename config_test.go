package amadeus

import (
	"os"
	"path/filepath"
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
