package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
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

func TestConfigShow(t *testing.T) {
	// given: a directory with valid config
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	os.MkdirAll(gateDir, 0755)
	os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte(`
lang: "en"
full_check:
  interval: 20
`), 0644)

	var stdout, stderr bytes.Buffer
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"config", "show", dir})
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("config show failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "lang: en") {
		t.Errorf("expected 'lang: en' in output, got:\n%s", out)
	}
}

func TestConfigSet_Lang(t *testing.T) {
	// given
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	os.MkdirAll(gateDir, 0755)
	os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte(`lang: "ja"`), 0644)

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"config", "set", "lang", "en", dir})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	// verify
	cfg, _ := loadConfig(filepath.Join(gateDir, "config.yaml"))
	if cfg.Lang != "en" {
		t.Errorf("expected lang 'en', got %q", cfg.Lang)
	}
}

func TestConfigSet_InvalidKey(t *testing.T) {
	// given
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	os.MkdirAll(gateDir, 0755)
	os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte(`lang: "ja"`), 0644)

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"config", "set", "bad.key", "value", dir})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Error("expected error for invalid config key")
	}
}

func TestConfigSet_InvalidLang(t *testing.T) {
	// given
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	os.MkdirAll(gateDir, 0755)
	os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte(`lang: "ja"`), 0644)

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"config", "set", "lang", "fr", dir})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Error("expected error for invalid lang value")
	}
}

func TestLoadConfig_FileNotFound_ReturnsDefault(t *testing.T) {
	cfg, err := loadConfig("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg.Weights.ADRIntegrity != 0.4 {
		t.Errorf("expected default ADR weight 0.4, got %f", cfg.Weights.ADRIntegrity)
	}
}
