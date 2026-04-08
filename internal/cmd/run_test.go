package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCmd_BootstrapWithInit(t *testing.T) {
	// given: initialized repo with .gate/ structure
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	for _, sub := range []string{".run", "events", "archive", "insights", "inbox", "outbox"} {
		os.MkdirAll(filepath.Join(gateDir, sub), 0755)
	}

	// git init (required by git client)
	gitInit := exec.Command("git", "init")
	gitInit.Dir = dir
	if out, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	// when: run with --dry-run --full (skip claude, skip waiting)
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"run", "--dry-run", "--full", dir})
	rootCmd.SetOut(&strings.Builder{})
	rootCmd.SetErr(&strings.Builder{})

	err := rootCmd.Execute()

	// then: should NOT fail with "not initialized"
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not initialized") {
			t.Fatalf("bootstrap failed: %v", err)
		}
		// Other errors (e.g. no commits) are acceptable — bootstrap passed
		t.Logf("run completed with non-init error (bootstrap OK): %v", err)
	}
}

func TestRunCmd_FailsWithoutInit(t *testing.T) {
	// given: empty directory without .gate/
	dir := t.TempDir()

	// when
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"run", dir})
	rootCmd.SetOut(&strings.Builder{})
	rootCmd.SetErr(&strings.Builder{})

	err := rootCmd.Execute()

	// then: should fail with "not initialized"
	if err == nil {
		t.Fatal("expected error for uninitalized repo")
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("expected 'not initialized' error, got: %v", err)
	}
}
