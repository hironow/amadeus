package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/hironow/amadeus/internal/domain"
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

func TestConfigSet_PerAxisOverride(t *testing.T) {
	cases := []struct {
		key   string
		value string
		check func(t *testing.T, dir string)
	}{
		{
			key:   "per_axis_override.adr_integrity_force_high",
			value: "75",
			check: func(t *testing.T, dir string) {
				cfg, _ := loadConfig(filepath.Join(dir, ".gate", "config.yaml"))
				if cfg.PerAxisOverride.ADRForceHigh != 75 {
					t.Errorf("expected ADRForceHigh 75, got %d", cfg.PerAxisOverride.ADRForceHigh)
				}
			},
		},
		{
			key:   "per_axis_override.dod_fulfillment_force_high",
			value: "80",
			check: func(t *testing.T, dir string) {
				cfg, _ := loadConfig(filepath.Join(dir, ".gate", "config.yaml"))
				if cfg.PerAxisOverride.DoDForceHigh != 80 {
					t.Errorf("expected DoDForceHigh 80, got %d", cfg.PerAxisOverride.DoDForceHigh)
				}
			},
		},
		{
			key:   "per_axis_override.dependency_integrity_force_medium",
			value: "50",
			check: func(t *testing.T, dir string) {
				cfg, _ := loadConfig(filepath.Join(dir, ".gate", "config.yaml"))
				if cfg.PerAxisOverride.DepForceMedium != 50 {
					t.Errorf("expected DepForceMedium 50, got %d", cfg.PerAxisOverride.DepForceMedium)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			// given
			dir := t.TempDir()
			gateDir := filepath.Join(dir, ".gate")
			os.MkdirAll(gateDir, 0755)
			os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte(`lang: "ja"`), 0644)

			rootCmd := NewRootCommand()
			rootCmd.SetArgs([]string{"config", "set", tc.key, tc.value, dir})
			rootCmd.SetOut(&bytes.Buffer{})
			rootCmd.SetErr(&bytes.Buffer{})

			// when
			err := rootCmd.Execute()

			// then
			if err != nil {
				t.Fatalf("config set %s failed: %v", tc.key, err)
			}
			tc.check(t, dir)
		})
	}
}

func TestConfigSet_PerAxisOverride_InvalidValue(t *testing.T) {
	keys := []string{
		"per_axis_override.adr_integrity_force_high",
		"per_axis_override.dod_fulfillment_force_high",
		"per_axis_override.dependency_integrity_force_medium",
	}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			// given
			dir := t.TempDir()
			gateDir := filepath.Join(dir, ".gate")
			os.MkdirAll(gateDir, 0755)
			os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte(`lang: "ja"`), 0644)

			rootCmd := NewRootCommand()
			rootCmd.SetArgs([]string{"config", "set", key, "notanumber", dir})
			rootCmd.SetOut(&bytes.Buffer{})
			rootCmd.SetErr(&bytes.Buffer{})

			// when
			err := rootCmd.Execute()

			// then
			if err == nil {
				t.Errorf("expected error for non-integer value on %s", key)
			}
		})
	}
}

func TestConfigSet_ClaudeCmd(t *testing.T) {
	// given
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	os.MkdirAll(gateDir, 0755)
	os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte(`lang: "ja"`), 0644)

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"config", "set", "claude_cmd", "custom-claude", dir})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("config set claude_cmd failed: %v", err)
	}

	// verify
	cfg, _ := loadConfig(filepath.Join(gateDir, "config.yaml"))
	if cfg.ClaudeCmd != "custom-claude" {
		t.Errorf("expected ClaudeCmd 'custom-claude', got %q", cfg.ClaudeCmd)
	}
}

func TestConfigSet_ClaudeCmd_Unit(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()

	// when
	err := setAmadeusConfigField(&cfg, "claude_cmd", "custom-claude")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ClaudeCmd != "custom-claude" {
		t.Errorf("ClaudeCmd = %q, want 'custom-claude'", cfg.ClaudeCmd)
	}
}

func TestConfigSet_Model(t *testing.T) {
	// given
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	os.MkdirAll(gateDir, 0755)
	os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte(`lang: "ja"`), 0644)

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"config", "set", "model", "sonnet", dir})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("config set model failed: %v", err)
	}

	// verify
	cfg, _ := loadConfig(filepath.Join(gateDir, "config.yaml"))
	if cfg.Model != "sonnet" {
		t.Errorf("expected Model 'sonnet', got %q", cfg.Model)
	}
}

func TestConfigSet_TimeoutSec(t *testing.T) {
	// given
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	os.MkdirAll(gateDir, 0755)
	os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte(`lang: "ja"`), 0644)

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"config", "set", "timeout_sec", "600", dir})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("config set timeout_sec failed: %v", err)
	}

	// verify
	cfg, _ := loadConfig(filepath.Join(gateDir, "config.yaml"))
	if cfg.TimeoutSec != 600 {
		t.Errorf("expected TimeoutSec 600, got %d", cfg.TimeoutSec)
	}
}

func TestConfigSet_TimeoutSec_Invalid(t *testing.T) {
	// given
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	os.MkdirAll(gateDir, 0755)
	os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte(`lang: "ja"`), 0644)

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"config", "set", "timeout_sec", "-5", dir})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Error("expected error for negative timeout_sec")
	}
}

func TestConfig_SaveLoadRoundTrip_AllFields(t *testing.T) {
	// given: DefaultConfig marshalled to YAML file
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	original := domain.DefaultConfig()
	data, err := yaml.Marshal(&original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// when: loadConfig from that file
	loaded, err := loadConfig(configPath)

	// then: no error
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	// verify key fields survive round-trip
	if loaded.Lang != "ja" {
		t.Errorf("Lang: expected 'ja', got %q", loaded.Lang)
	}
	if loaded.ClaudeCmd != "claude" {
		t.Errorf("ClaudeCmd: expected 'claude', got %q", loaded.ClaudeCmd)
	}
	if loaded.Model != "opus" {
		t.Errorf("Model: expected 'opus', got %q", loaded.Model)
	}
	if loaded.TimeoutSec != 1980 {
		t.Errorf("TimeoutSec: expected 1980, got %d", loaded.TimeoutSec)
	}

	// Weights
	if loaded.Weights.ADRIntegrity != 0.4 {
		t.Errorf("Weights.ADRIntegrity: expected 0.4, got %f", loaded.Weights.ADRIntegrity)
	}
	if loaded.Weights.DoDFulfillment != 0.3 {
		t.Errorf("Weights.DoDFulfillment: expected 0.3, got %f", loaded.Weights.DoDFulfillment)
	}
	if loaded.Weights.DependencyIntegrity != 0.2 {
		t.Errorf("Weights.DependencyIntegrity: expected 0.2, got %f", loaded.Weights.DependencyIntegrity)
	}
	if loaded.Weights.ImplicitConstraints != 0.1 {
		t.Errorf("Weights.ImplicitConstraints: expected 0.1, got %f", loaded.Weights.ImplicitConstraints)
	}

	// Thresholds
	if loaded.Thresholds.LowMax != 0.25 {
		t.Errorf("Thresholds.LowMax: expected 0.25, got %f", loaded.Thresholds.LowMax)
	}
	if loaded.Thresholds.MediumMax != 0.5 {
		t.Errorf("Thresholds.MediumMax: expected 0.5, got %f", loaded.Thresholds.MediumMax)
	}

	// FullCheck
	if loaded.FullCheck.Interval != 10 {
		t.Errorf("FullCheck.Interval: expected 10, got %d", loaded.FullCheck.Interval)
	}
	if loaded.FullCheck.OnDivergenceJump != 0.15 {
		t.Errorf("FullCheck.OnDivergenceJump: expected 0.15, got %f", loaded.FullCheck.OnDivergenceJump)
	}

	// Convergence
	if loaded.Convergence.WindowDays != 14 {
		t.Errorf("Convergence.WindowDays: expected 14, got %d", loaded.Convergence.WindowDays)
	}
	if loaded.Convergence.Threshold != 3 {
		t.Errorf("Convergence.Threshold: expected 3, got %d", loaded.Convergence.Threshold)
	}
	if loaded.Convergence.EscalationMultiplier != 2 {
		t.Errorf("Convergence.EscalationMultiplier: expected 2, got %d", loaded.Convergence.EscalationMultiplier)
	}

	// WaitTimeout
	if loaded.WaitTimeout != domain.DefaultWaitTimeout {
		t.Errorf("WaitTimeout: expected %v, got %v", domain.DefaultWaitTimeout, loaded.WaitTimeout)
	}

	// verify Computed is zero-value after round-trip of defaults
	if loaded.Computed != (domain.ComputedConfig{}) {
		t.Errorf("Computed: expected zero-value, got %+v", loaded.Computed)
	}
}

func TestConfigSet_WaitTimeout_Valid(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()

	// when
	err := setAmadeusConfigField(&cfg, "wait_timeout", "10m")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WaitTimeout != 10*time.Minute {
		t.Errorf("WaitTimeout = %v, want 10m", cfg.WaitTimeout)
	}
}

func TestConfigSet_WaitTimeout_Zero(t *testing.T) {
	// given: zero disables timeout (infinite wait)
	cfg := domain.DefaultConfig()

	// when
	err := setAmadeusConfigField(&cfg, "wait_timeout", "0s")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WaitTimeout != 0 {
		t.Errorf("WaitTimeout = %v, want 0", cfg.WaitTimeout)
	}
}

func TestConfigSet_WaitTimeout_Negative(t *testing.T) {
	// given: negative disables waiting mode
	cfg := domain.DefaultConfig()

	// when
	err := setAmadeusConfigField(&cfg, "wait_timeout", "-1s")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WaitTimeout != -1*time.Second {
		t.Errorf("WaitTimeout = %v, want -1s", cfg.WaitTimeout)
	}
}

func TestConfigSet_WaitTimeout_Invalid(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()

	// when
	err := setAmadeusConfigField(&cfg, "wait_timeout", "not-a-duration")

	// then
	if err == nil {
		t.Error("expected error for invalid wait_timeout")
	}
	if !strings.Contains(err.Error(), "invalid wait_timeout") {
		t.Errorf("expected 'invalid wait_timeout' in error, got: %v", err)
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
