//go:build scenario

package scenario_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// TestScenario_L1_EnvPrefixedClaudeCmd verifies that env-prefixed claude_cmd
// (e.g. "CLAUDE_CONFIG_DIR=/tmp/test-config claude") correctly propagates
// the environment variable to the spawned claude process.
func TestScenario_L1_EnvPrefixedClaudeCmd(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "minimal")

	// given: env log directory for fake-claude to write captured env vars
	envLogDir := t.TempDir()
	ws.Env = append(ws.Env, "FAKE_CLAUDE_ENV_LOG_DIR="+envLogDir)

	// given: override claude_cmd in .gate/config.yaml with env-prefixed command
	cfgPath := filepath.Join(ws.RepoPath, ".gate", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read amadeus config: %v", err)
	}

	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse amadeus config: %v", err)
	}

	// Build the env-prefixed claude command using the fake-claude binary from binDir
	fakeClaude := filepath.Join(ws.BinDir, "claude")
	cfg["claude_cmd"] = "CLAUDE_CONFIG_DIR=/tmp/test-config " + fakeClaude
	out, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal amadeus config: %v", err)
	}
	if err := os.WriteFile(cfgPath, out, 0o644); err != nil {
		t.Fatalf("write amadeus config: %v", err)
	}

	// when: run amadeus doctor (doctor invokes claude --version, which triggers env logging)
	// doctor may return non-zero exit code due to git remote check (test repo has no remote),
	// but claude checks still run and produce env logs.
	_ = ws.RunAmadeus(t, ctx, "doctor", ws.RepoPath)

	// then: fake-claude should have written env log files
	entries, err := os.ReadDir(envLogDir)
	if err != nil {
		t.Fatalf("read env log dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no env log files written by fake-claude")
	}

	// Read the first env log and verify CLAUDE_CONFIG_DIR was propagated
	logPath := filepath.Join(envLogDir, entries[0].Name())
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read env log: %v", err)
	}

	var envLog map[string]any
	if err := json.Unmarshal(logData, &envLog); err != nil {
		t.Fatalf("parse env log JSON: %v", err)
	}

	configDir, ok := envLog["CLAUDE_CONFIG_DIR"].(string)
	if !ok {
		t.Fatalf("CLAUDE_CONFIG_DIR not found in env log; got: %s", string(logData))
	}
	if configDir != "/tmp/test-config" {
		t.Errorf("CLAUDE_CONFIG_DIR: want %q, got %q", "/tmp/test-config", configDir)
	}
}
