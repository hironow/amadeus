package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/cmd"
)

func TestValidateCmd_SubcommandExists(t *testing.T) {
	// given
	root := cmd.NewRootCommand()

	// when
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "validate" {
			found = true
			break
		}
	}

	// then
	if !found {
		t.Fatal("validate subcommand not found")
	}
}

func TestValidateCmd_RejectsTooManyArgs(t *testing.T) {
	// given
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"validate", "arg1", "arg2"})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestValidateCmd_ConfigNotFound(t *testing.T) {
	// given: temp dir without .gate/config.yaml
	dir := t.TempDir()
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"validate", dir})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %s", err.Error())
	}
}

func TestValidateCmd_ValidConfig(t *testing.T) {
	// given: valid config file
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	if err := os.MkdirAll(gateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configContent := `lang: en
tracker:
  team: MY
  project: TestProject
`
	if err := os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"validate", dir})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("validate valid config failed: %v", err)
	}
	if !strings.Contains(stderr.String(), "[OK]") {
		t.Errorf("expected [OK] in stderr, got: %q", stderr.String())
	}
}

func TestValidateCmd_InvalidConfig(t *testing.T) {
	// given: config with invalid lang
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	if err := os.MkdirAll(gateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configContent := `lang: fr
tracker:
  team: ""
`
	if err := os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"validate", dir})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	if !strings.Contains(stderr.String(), "[FAIL]") {
		t.Errorf("expected [FAIL] in stderr, got: %q", stderr.String())
	}
}

func TestValidateCmd_ExplicitConfigFlag(t *testing.T) {
	// given: valid config at custom path (not in .gate/)
	dir := t.TempDir()
	configContent := `lang: en
tracker:
  team: MY
  project: TestProject
`
	cfgPath := filepath.Join(dir, "custom-config.yaml")
	if err := os.WriteFile(cfgPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"validate", "--config", cfgPath})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("validate with --config flag failed: %v", err)
	}
	if !strings.Contains(stderr.String(), "[OK]") {
		t.Errorf("expected [OK] in stderr, got: %q", stderr.String())
	}
}
