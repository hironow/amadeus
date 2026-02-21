//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2E_Check_DryRun(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	stdout, _, err := runCmd(t, dir, "check", "--dry-run")
	if err != nil {
		t.Fatalf("check --dry-run: %v", err)
	}
	// Dry-run outputs the prompt text, not JSON
	if stdout == "" {
		t.Error("expected prompt output in dry-run mode")
	}
	// Should contain template markers
	if !strings.Contains(stdout, "calibration") && !strings.Contains(stdout, "FULL") {
		t.Logf("dry-run output (truncated): %.200s", stdout)
	}
}

func TestE2E_Check_FullCalibration(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	// Force full calibration — fake-claude returns fullCalibrationResponse (with D-Mails)
	stdout, stderr, err := runCmd(t, dir, "check", "--full", "--json")
	// Exit code 2 = drift detected (D-Mails generated)
	assertExitCode(t, err, 2)

	var result struct {
		Divergence float64 `json:"divergence"`
		Delta      float64 `json:"delta"`
		Axes       map[string]struct {
			Score   int    `json:"score"`
			Details string `json:"details"`
		} `json:"axes"`
		ImpactRadius []struct {
			Area   string `json:"area"`
			Impact string `json:"impact"`
			Detail string `json:"detail"`
		} `json:"impact_radius"`
		DMails []struct {
			Name     string   `json:"name"`
			Kind     string   `json:"kind"`
			Severity string   `json:"severity"`
			Issues   []string `json:"issues"`
		} `json:"dmails"`
		ConvergenceAlerts []any `json:"convergence_alerts"`
	}
	parseJSONOutput(t, stdout, &result)

	// Verify divergence was scored
	if result.Divergence <= 0 {
		t.Errorf("expected positive divergence, got %f", result.Divergence)
	}

	// Verify axes populated
	for _, axis := range []string{"adr_integrity", "dod_fulfillment", "dependency_integrity", "implicit_constraints"} {
		if _, ok := result.Axes[axis]; !ok {
			t.Errorf("missing axis: %s", axis)
		}
	}

	// Verify D-Mails generated
	if len(result.DMails) == 0 {
		t.Error("expected D-Mails from full calibration")
	}

	// Verify impact radius populated
	if len(result.ImpactRadius) == 0 {
		t.Error("expected impact_radius entries")
	}

	// Verify files created on disk
	archiveFiles := listDir(t, filepath.Join(dir, ".gate", "archive"))
	if len(archiveFiles) == 0 {
		t.Error("expected D-Mail files in archive/")
	}

	// Verify latest.json saved
	assertFileExists(t, filepath.Join(dir, ".gate", ".run", "latest.json"))

	// Verify history saved
	historyFiles := listDir(t, filepath.Join(dir, ".gate", "history"))
	if len(historyFiles) == 0 {
		t.Error("expected history entry")
	}

	_ = stderr
}

func TestE2E_Check_FullCalibration_TextOutput(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	stdout, _, err := runCmd(t, dir, "check", "--full")
	assertExitCode(t, err, 2)

	// Text output should contain divergence info and D-Mail listing
	if !strings.Contains(stdout, "Divergence") {
		t.Errorf("expected 'Divergence' in text output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "D-Mails") {
		t.Errorf("expected 'D-Mails' in text output, got: %s", stdout)
	}
}

func TestE2E_Check_FullCalibration_QuietOutput(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	stdout, _, err := runCmd(t, dir, "check", "--full", "--quiet")
	assertExitCode(t, err, 2)

	// Quiet output is a single summary line
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line in quiet output, got %d: %q", len(lines), stdout)
	}
	if !strings.Contains(stdout, "D-Mail") {
		t.Errorf("expected 'D-Mail' in quiet output, got: %s", stdout)
	}
}

func TestE2E_Check_DiffCheck_NoDrift(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	// First run: full calibration to establish baseline
	_, _, err := runCmd(t, dir, "check", "--full", "--json")
	assertExitCode(t, err, 2) // has D-Mails from full calibration

	// Second run: diff check — fake-claude returns defaultCleanResponse (no D-Mails)
	// Since no new commits since last check, Reading Steiner detects no shift
	stdout, _, err := runCmd(t, dir, "check", "--json")
	if err != nil {
		t.Fatalf("second check: %v\nstdout: %s", err, stdout)
	}

	var result struct {
		Divergence float64 `json:"divergence"`
		DMails     []any   `json:"dmails"`
	}
	parseJSONOutput(t, stdout, &result)

	if len(result.DMails) != 0 {
		t.Errorf("expected no D-Mails on diff check with no changes, got %d", len(result.DMails))
	}
}

func TestE2E_Check_SeverityRouting_High(t *testing.T) {
	dir := initTestRepo(t)
	// Configure thresholds so that fullCalibrationResponse triggers HIGH severity
	cfg := defaultTestConfig()
	cfg["thresholds"] = map[string]any{
		"low_max":    0.05,
		"medium_max": 0.10,
	}
	writeConfig(t, dir, cfg)

	_, _, err := runCmd(t, dir, "check", "--full", "--json")
	assertExitCode(t, err, 2)

	// HIGH severity D-Mails should be in pending/
	pendingFiles := listDir(t, filepath.Join(dir, ".gate", "pending"))
	if len(pendingFiles) == 0 {
		t.Error("expected D-Mail files in pending/ (HIGH severity)")
	}
}

func TestE2E_Check_SeverityRouting_Low(t *testing.T) {
	dir := initTestRepo(t)
	// Configure thresholds so that fullCalibrationResponse triggers LOW severity
	cfg := defaultTestConfig()
	cfg["thresholds"] = map[string]any{
		"low_max":    0.90,
		"medium_max": 0.95,
	}
	writeConfig(t, dir, cfg)

	_, _, err := runCmd(t, dir, "check", "--full", "--json")
	assertExitCode(t, err, 2)

	// LOW severity D-Mails should be in outbox/ (auto-sent)
	outboxFiles := listDir(t, filepath.Join(dir, ".gate", "outbox"))
	if len(outboxFiles) == 0 {
		t.Error("expected D-Mail files in outbox/ (LOW severity auto-sent)")
	}

	// Nothing in pending/
	pendingFiles := listDir(t, filepath.Join(dir, ".gate", "pending"))
	if len(pendingFiles) != 0 {
		t.Errorf("expected no files in pending/ for LOW severity, got %d", len(pendingFiles))
	}
}

func TestE2E_Check_HistoryAccumulates(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	// Run two checks
	runCmd(t, dir, "check", "--full", "--json")

	// Add a commit to trigger a shift on second run
	f := filepath.Join(dir, "new.go")
	os.WriteFile(f, []byte("package main\n"), 0o644)
	gitAdd := exec.Command("git", "add", ".")
	gitAdd.Dir = dir
	gitAdd.Run()
	gitCommit := exec.Command("git", "commit", "-m", "second")
	gitCommit.Dir = dir
	gitCommit.Run()

	runCmd(t, dir, "check", "--full", "--json")

	historyFiles := listDir(t, filepath.Join(dir, ".gate", "history"))
	if len(historyFiles) < 2 {
		t.Errorf("expected at least 2 history files, got %d", len(historyFiles))
	}
}

func TestE2E_Check_LatestJSON(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	runCmd(t, dir, "check", "--full", "--json")

	latestPath := filepath.Join(dir, ".gate", ".run", "latest.json")
	var latest struct {
		Commit     string  `json:"commit"`
		Type       string  `json:"type"`
		Divergence float64 `json:"divergence"`
	}
	readJSON(t, latestPath, &latest)

	if latest.Commit == "" {
		t.Error("expected commit in latest.json")
	}
	if latest.Type != "full" {
		t.Errorf("expected type=full, got %s", latest.Type)
	}
}

func TestE2E_Check_DMailIssuesField(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	stdout, _, err := runCmd(t, dir, "check", "--full", "--json")
	assertExitCode(t, err, 2)

	var result struct {
		DMails []struct {
			Issues []string `json:"issues"`
		} `json:"dmails"`
	}
	parseJSONOutput(t, stdout, &result)

	// fullCalibrationResponse fixture includes issues: ["MY-100"]
	found := false
	for _, d := range result.DMails {
		if len(d.Issues) > 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected at least one D-Mail with issues field populated")
	}
}

func TestE2E_Check_BaselineSaved(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	runCmd(t, dir, "check", "--full", "--json")

	// Full check should save baseline
	baselinePath := filepath.Join(dir, ".gate", ".run", "baseline.json")
	assertFileExists(t, baselinePath)

	var baseline struct {
		Type string `json:"type"`
	}
	readJSON(t, baselinePath, &baseline)
	if baseline.Type != "full" {
		t.Errorf("expected baseline type=full, got %s", baseline.Type)
	}
}

func TestE2E_Log_AfterCheck(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	runCmd(t, dir, "check", "--full", "--json")

	// Text log
	stdout, _, err := runCmd(t, dir, "log")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if !strings.Contains(stdout, "History") {
		t.Errorf("expected 'History' in log output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "D-Mails") {
		t.Errorf("expected 'D-Mails' in log output, got: %s", stdout)
	}

	// JSON log
	jstdout, _, err := runCmd(t, dir, "log", "--json")
	if err != nil {
		t.Fatalf("log --json: %v", err)
	}
	var jresult struct {
		History []any `json:"history"`
		DMails  []any `json:"dmails"`
	}
	if err := json.Unmarshal([]byte(jstdout), &jresult); err != nil {
		t.Fatalf("parse log JSON: %v", err)
	}
	if len(jresult.History) == 0 {
		t.Error("expected history entries in log JSON")
	}
	if len(jresult.DMails) == 0 {
		t.Error("expected dmail entries in log JSON")
	}
}
