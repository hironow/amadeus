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

// TestE2E_FullPipeline exercises the complete amadeus workflow:
// init → check → resolve → sync → mark-commented → sync (empty)
func TestE2E_FullPipeline(t *testing.T) {
	dir := initTestRepo(t)
	cfg := defaultTestConfig()
	// Set thresholds so fullCalibrationResponse triggers HIGH severity
	cfg["thresholds"] = map[string]any{
		"low_max":    0.05,
		"medium_max": 0.10,
	}
	writeConfig(t, dir, cfg)

	// Step 1: Run check → generates D-Mails (exit code 2)
	checkStdout, _, err := runCmd(t, dir, "check", "--full", "--json")
	assertExitCode(t, err, 2)

	var checkResult struct {
		DMails []struct {
			Name   string   `json:"name"`
			Issues []string `json:"issues"`
		} `json:"dmails"`
	}
	parseJSONOutput(t, checkStdout, &checkResult)

	if len(checkResult.DMails) == 0 {
		t.Fatal("expected D-Mails from full calibration")
	}

	dmailName := checkResult.DMails[0].Name
	t.Logf("Generated D-Mail: %s", dmailName)

	// D-Mail should be in pending/ (HIGH severity)
	assertFileExists(t, filepath.Join(dir, ".gate", "pending", dmailName+".md"))

	// Step 2: Sync → should show pending comments
	syncStdout, _, err := runCmd(t, dir, "sync")
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	var syncResult struct {
		PendingComments []struct {
			DMail   string `json:"dmail"`
			IssueID string `json:"issue_id"`
		} `json:"pending_comments"`
	}
	parseJSONOutput(t, syncStdout, &syncResult)

	if len(syncResult.PendingComments) == 0 {
		t.Fatal("expected pending comments before resolve")
	}

	// Step 3: Resolve → approve with JSON output (includes CommentPayload)
	resolveStdout, _, err := runCmd(t, dir, "resolve", dmailName, "--approve", "--json")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	var resolveResults []struct {
		Name     string `json:"name"`
		Status   string `json:"status"`
		Comments []struct {
			IssueID string `json:"issue_id"`
			DMail   string `json:"dmail"`
		} `json:"comments"`
	}
	parseJSONOutput(t, resolveStdout, &resolveResults)

	if len(resolveResults) != 1 {
		t.Fatalf("expected 1 resolve result, got %d", len(resolveResults))
	}
	if resolveResults[0].Status != "approved" {
		t.Errorf("expected approved, got %s", resolveResults[0].Status)
	}

	// Step 4: mark-commented for each issue
	for _, comment := range resolveResults[0].Comments {
		_, _, err := runCmd(t, dir, "mark-commented", comment.DMail, comment.IssueID)
		if err != nil {
			t.Fatalf("mark-commented %s %s: %v", comment.DMail, comment.IssueID, err)
		}
	}

	// Step 5: Sync → should now be empty
	syncStdout2, _, err := runCmd(t, dir, "sync")
	if err != nil {
		t.Fatalf("sync after mark-commented: %v", err)
	}

	var syncResult2 struct {
		PendingComments []any `json:"pending_comments"`
	}
	parseJSONOutput(t, syncStdout2, &syncResult2)

	if len(syncResult2.PendingComments) != 0 {
		t.Errorf("expected 0 pending comments after marking all, got %d", len(syncResult2.PendingComments))
	}

	// Step 6: Log → should show history and D-Mails
	logStdout, _, err := runCmd(t, dir, "log", "--json")
	if err != nil {
		t.Fatalf("log: %v", err)
	}

	var logResult struct {
		History []any `json:"history"`
		DMails  []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"dmails"`
	}
	parseJSONOutput(t, logStdout, &logResult)

	if len(logResult.History) == 0 {
		t.Error("expected history entries")
	}
	if len(logResult.DMails) == 0 {
		t.Error("expected D-Mail entries in log")
	}

	// Verify the D-Mail shows as approved
	for _, d := range logResult.DMails {
		if d.Name == dmailName && d.Status != "approved" {
			t.Errorf("expected D-Mail %s status=approved in log, got %s", dmailName, d.Status)
		}
	}
}

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

// TestE2E_Pipeline_RejectAndLog exercises reject flow with reason tracking.
func TestE2E_Pipeline_RejectAndLog(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	seedDMails(t, dir, []seedDMailSpec{{
		Name:        "feedback-001",
		Kind:        "feedback",
		Description: "Not relevant",
		Severity:    "high",
		Issues:      []string{"MY-999"},
	}})

	// Reject with reason
	runCmd(t, dir, "resolve", "feedback-001", "--reject", "--reason", "False positive")

	// Verify rejected/ directory
	assertFileExists(t, filepath.Join(dir, ".gate", "rejected", "feedback-001.md"))
	assertFileNotExists(t, filepath.Join(dir, ".gate", "pending", "feedback-001.md"))

	// Log should show rejected status
	stdout, _, _ := runCmd(t, dir, "log", "--json")
	var logResult struct {
		DMails []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"dmails"`
	}
	parseJSONOutput(t, stdout, &logResult)

	for _, d := range logResult.DMails {
		if d.Name == "feedback-001" {
			if d.Status != "rejected" {
				t.Errorf("expected status=rejected, got %s", d.Status)
			}
			if d.Reason != "False positive" {
				t.Errorf("expected reason='False positive', got %s", d.Reason)
			}
		}
	}

	// Sync should still show pending comment (rejected D-Mail still has issues)
	syncStdout, _, _ := runCmd(t, dir, "sync")
	var syncResult struct {
		PendingComments []struct {
			Status string `json:"status"`
		} `json:"pending_comments"`
	}
	parseJSONOutput(t, syncStdout, &syncResult)

	if len(syncResult.PendingComments) != 1 {
		t.Errorf("expected 1 pending comment for rejected D-Mail, got %d", len(syncResult.PendingComments))
	}
	if syncResult.PendingComments[0].Status != "rejected" {
		t.Errorf("expected pending comment status=rejected, got %s", syncResult.PendingComments[0].Status)
	}
}

// TestE2E_Pipeline_HookInstallUninstall tests git hook lifecycle.
func TestE2E_Pipeline_HookInstallUninstall(t *testing.T) {
	dir := initTestRepo(t)

	// Install hook
	stdout, _, err := runCmd(t, dir, "install-hook")
	if err != nil {
		t.Fatalf("install-hook: %v\nstdout: %s", err, stdout)
	}
	if !strings.Contains(stdout, "Installed") {
		t.Errorf("expected 'Installed' in output, got: %s", stdout)
	}
	assertFileExists(t, filepath.Join(dir, ".git", "hooks", "post-merge"))

	// Uninstall hook
	stdout, _, err = runCmd(t, dir, "uninstall-hook")
	if err != nil {
		t.Fatalf("uninstall-hook: %v", err)
	}
	if !strings.Contains(stdout, "Removed") {
		t.Errorf("expected 'Removed' in output, got: %s", stdout)
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
		{"resolve", "x", "--approve"},
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
