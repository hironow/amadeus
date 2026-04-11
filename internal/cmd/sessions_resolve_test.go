package cmd

// white-box-reason: tests resolveSessionsDir which is an unexported helper used by sessions commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/spf13/cobra"
)

func newTestCommand(withConfig bool) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("path", "", "")
	if withConfig {
		cmd.Flags().String("config", "", "")
	}
	return cmd
}

func TestResolveSessionsDir_PathFlag(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, domain.StateDir), 0755)

	cmd := newTestCommand(false)
	cmd.Flags().Set("path", dir)

	repoRoot, stateDirPath, err := resolveSessionsDir(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repoRoot != dir {
		t.Errorf("repoRoot = %q, want %q", repoRoot, dir)
	}
	wantState := filepath.Join(dir, domain.StateDir)
	if stateDirPath != wantState {
		t.Errorf("stateDirPath = %q, want %q", stateDirPath, wantState)
	}
}

func TestResolveSessionsDir_ConfigFlag(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	os.MkdirAll(stateDir, 0755)
	configPath := filepath.Join(stateDir, "config.yaml")
	os.WriteFile(configPath, []byte("team: test\n"), 0644)

	cmd := newTestCommand(true)
	cmd.Flags().Set("config", configPath)

	repoRoot, stateDirPath, err := resolveSessionsDir(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repoRoot != dir {
		t.Errorf("repoRoot = %q, want %q", repoRoot, dir)
	}
	if stateDirPath != stateDir {
		t.Errorf("stateDirPath = %q, want %q", stateDirPath, stateDir)
	}
}

func TestResolveSessionsDir_CwdFallback(t *testing.T) {
	dir := t.TempDir()
	// Resolve symlinks to handle macOS /var → /private/var
	dir, _ = filepath.EvalSymlinks(dir)
	os.MkdirAll(filepath.Join(dir, domain.StateDir), 0755)

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(oldWd) })

	cmd := newTestCommand(false)
	repoRoot, _, err := resolveSessionsDir(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repoRoot != dir {
		t.Errorf("repoRoot = %q, want %q", repoRoot, dir)
	}
}

func TestResolveSessionsDir_PathOverridesConfig(t *testing.T) {
	pathDir := t.TempDir()
	os.MkdirAll(filepath.Join(pathDir, domain.StateDir), 0755)

	configDir := t.TempDir()
	configStateDir := filepath.Join(configDir, domain.StateDir)
	os.MkdirAll(configStateDir, 0755)
	configPath := filepath.Join(configStateDir, "config.yaml")
	os.WriteFile(configPath, []byte("team: test\n"), 0644)

	cmd := newTestCommand(true)
	cmd.Flags().Set("path", pathDir)
	cmd.Flags().Set("config", configPath)

	repoRoot, _, err := resolveSessionsDir(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repoRoot != pathDir {
		t.Errorf("repoRoot = %q, want %q (--path should override --config)", repoRoot, pathDir)
	}
}

func TestResolveSessionsDir_MissingStateDir(t *testing.T) {
	dir := t.TempDir() // no StateDir inside

	cmd := newTestCommand(false)
	cmd.Flags().Set("path", dir)

	_, _, err := resolveSessionsDir(cmd)
	if err == nil {
		t.Fatal("expected error for missing state dir")
	}
	if !strings.Contains(err.Error(), "state directory not found:") {
		t.Errorf("error = %q, want 'state directory not found:' prefix", err)
	}
	if !strings.Contains(err.Error(), "run 'amadeus init' first") {
		t.Errorf("error = %q, want '(run amadeus init first)' suffix", err)
	}
}

func TestResolveSessionsDir_ErrorMessageFormat(t *testing.T) {
	dir := t.TempDir()

	cmd := newTestCommand(false)
	cmd.Flags().Set("path", dir)

	_, _, err := resolveSessionsDir(cmd)
	if err == nil {
		t.Fatal("expected error")
	}
	wantSuffix := "(run 'amadeus init' first)"
	if !strings.Contains(err.Error(), wantSuffix) {
		t.Errorf("error %q missing %q", err.Error(), wantSuffix)
	}
}
