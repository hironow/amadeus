//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestE2E_Pipeline_Convergence exercises convergence detection with seeded D-Mails.
func TestE2E_Pipeline_Convergence(t *testing.T) {
	dir := initTestRepo(t)
	cfg := defaultTestConfig()
	cfg["convergence"] = map[string]any{
		"window_days": 14,
		"threshold":   3,
	}
	cfg["thresholds"] = map[string]any{
		"low_max":    0.90,
		"medium_max": 0.95,
	}
	writeConfig(t, dir, cfg)

	// Seed 6 feedback D-Mails targeting the same file (triggers HIGH convergence)
	// threshold=3, so 6 >= threshold*2 → HIGH severity → generates convergence D-Mail
	now := time.Now().UTC()
	for i := 1; i <= 6; i++ {
		name := seedName("feedback", i)
		seedDMails(t, dir, []seedDMailSpec{{
			Name:        name,
			Kind:        "feedback",
			Description: "Issue in auth/session.go",
			Severity:    "low",
			Targets:     []string{"auth/session.go"},
			Metadata: map[string]string{
				"created_at": now.Add(-time.Duration(i) * 24 * time.Hour).Format(time.RFC3339),
			},
		}})
	}

	// Run check — convergence detection runs on all archive D-Mails
	stdout, _, err := runCmd(t, dir, "check", "--full", "--json")
	assertExitCode(t, err, 2)

	var result struct {
		ConvergenceAlerts []struct {
			Target   string `json:"target"`
			Count    int    `json:"count"`
			Severity string `json:"severity"`
		} `json:"convergence_alerts"`
		DMails []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"dmails"`
	}
	parseJSONOutput(t, stdout, &result)

	// Verify convergence alert was generated
	if len(result.ConvergenceAlerts) == 0 {
		t.Error("expected convergence alerts for 3 D-Mails targeting same file")
	}

	// Verify convergence D-Mail was created
	hasConvergence := false
	for _, d := range result.DMails {
		if d.Kind == "convergence" {
			hasConvergence = true
		}
	}
	if !hasConvergence {
		t.Error("expected a convergence D-Mail to be generated")
	}

	// Verify convergence D-Mail exists on disk
	archiveFiles := listDir(t, filepath.Join(dir, ".gate", "archive"))
	convergenceFound := false
	for _, f := range archiveFiles {
		if strings.HasPrefix(f, "convergence-") {
			convergenceFound = true
		}
	}
	if !convergenceFound {
		t.Error("expected convergence D-Mail file in archive/")
	}
}

// TestE2E_Pipeline_HookInstallUninstall tests git hook lifecycle.
func TestE2E_Pipeline_HookInstallUninstall(t *testing.T) {
	dir := initTestRepo(t)

	// Install hook
	_, stderr, err := runCmd(t, dir, "install-hook")
	if err != nil {
		t.Fatalf("install-hook: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stderr, "Installed") {
		t.Errorf("expected 'Installed' in stderr, got: %s", stderr)
	}
	assertFileExists(t, filepath.Join(dir, ".git", "hooks", "post-merge"))

	// Uninstall hook
	_, stderr, err = runCmd(t, dir, "uninstall-hook")
	if err != nil {
		t.Fatalf("uninstall-hook: %v", err)
	}
	if !strings.Contains(stderr, "Removed") {
		t.Errorf("expected 'Removed' in stderr, got: %s", stderr)
	}
	assertFileNotExists(t, filepath.Join(dir, ".git", "hooks", "post-merge"))
}

// TestE2E_Pipeline_MultiCheckWithDivergenceHistory runs multiple checks and verifies history.
func TestE2E_Pipeline_MultiCheckWithDivergenceHistory(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	// First check (full)
	runCmd(t, dir, "check", "--full", "--json")

	// Add commit
	os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n"), 0o644)
	gitAdd := exec.Command("git", "add", ".")
	gitAdd.Dir = dir
	gitAdd.Run()
	gitCommit := exec.Command("git", "commit", "-m", "add x")
	gitCommit.Dir = dir
	gitCommit.Run()

	// Second check (full again)
	runCmd(t, dir, "check", "--full", "--json")

	// Verify log shows both entries
	stdout, _, _ := runCmd(t, dir, "log", "--json")
	var logResult struct {
		History []struct {
			Type string `json:"type"`
		} `json:"history"`
	}
	parseJSONOutput(t, stdout, &logResult)

	if len(logResult.History) < 2 {
		t.Errorf("expected at least 2 history entries, got %d", len(logResult.History))
	}
}

// TestE2E_Pipeline_GateNotFound tests commands that require .gate/.
func TestE2E_Pipeline_GateNotFound(t *testing.T) {
	dir := t.TempDir()

	// These commands should fail without .gate/
	for _, cmd := range [][]string{
		{"sync"},
		{"log"},
		{"mark-commented", "x", "y"},
	} {
		_, _, err := runCmd(t, dir, cmd...)
		if err == nil {
			t.Errorf("expected error for %v without .gate/", cmd)
		}
	}
}

func seedName(kind string, n int) string {
	return fmt.Sprintf("%s-%03d", kind, n)
}
