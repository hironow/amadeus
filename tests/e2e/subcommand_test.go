//go:build e2e

package e2e

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestE2E_Version(t *testing.T) {
	stdout, _, err := runCmd(t, t.TempDir(), "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if !strings.Contains(stdout, "amadeus") {
		t.Errorf("expected 'amadeus' in version output, got: %s", stdout)
	}
}

func TestE2E_VersionJSON(t *testing.T) {
	stdout, _, err := runCmd(t, t.TempDir(), "version", "--json")
	if err != nil {
		t.Fatalf("version --json: %v", err)
	}
	var v map[string]string
	parseJSONOutput(t, stdout, &v)
	for _, key := range []string{"version", "commit", "date"} {
		if _, ok := v[key]; !ok {
			t.Errorf("missing key %q in JSON output", key)
		}
	}
}

func TestE2E_Help(t *testing.T) {
	stdout, _, err := runCmd(t, t.TempDir(), "--help")
	if err != nil {
		t.Fatalf("--help: %v", err)
	}
	for _, sub := range []string{"init", "check", "resolve", "sync", "doctor", "log", "validate", "mark-commented", "archive-prune", "version"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("expected %q in help output", sub)
		}
	}
}

func TestE2E_UnknownCommand(t *testing.T) {
	_, _, err := runCmd(t, t.TempDir(), "nonexistent-cmd")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestE2E_NoSubcommand(t *testing.T) {
	_, _, err := runCmd(t, t.TempDir())
	if err == nil {
		t.Fatal("expected error when no subcommand given")
	}
}

func TestE2E_Init(t *testing.T) {
	dir := initTestRepo(t)

	// Verify .gate structure
	for _, sub := range []string{".run", "history", "outbox", "inbox", "archive", "pending", "rejected"} {
		assertFileExists(t, dir+"/.gate/"+sub)
	}
	assertFileExists(t, dir+"/.gate/config.yaml")
	assertFileExists(t, dir+"/.gate/.gitignore")
	assertFileExists(t, dir+"/.gate/skills/dmail-sendable/SKILL.md")
	assertFileExists(t, dir+"/.gate/skills/dmail-readable/SKILL.md")
}

func TestE2E_Init_Idempotent(t *testing.T) {
	dir := initTestRepo(t)
	// Running init again should not fail
	_, _, err := runCmd(t, dir, "init")
	if err != nil {
		t.Fatalf("second init: %v", err)
	}
}

func TestE2E_Validate_ValidConfig(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	stdout, _, err := runCmd(t, dir, "validate")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !strings.Contains(stdout, "[OK]") {
		t.Errorf("expected [OK] in output, got: %s", stdout)
	}
}

func TestE2E_Validate_InvalidConfig(t *testing.T) {
	dir := initTestRepo(t)
	cfg := defaultTestConfig()
	// Break weights sum
	cfg["weights"] = map[string]any{
		"adr_integrity":        0.50,
		"dod_fulfillment":      0.50,
		"dependency_integrity": 0.50,
		"implicit_constraints": 0.50,
	}
	writeConfig(t, dir, cfg)

	_, _, err := runCmd(t, dir, "validate")
	if err == nil {
		t.Fatal("expected validation error for bad weights sum")
	}
}

func TestE2E_Doctor(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	stdout, _, err := runCmd(t, dir, "doctor")
	// doctor may pass or fail depending on environment, but should produce output
	_ = err
	if stdout == "" {
		t.Error("expected doctor output")
	}
}

func TestE2E_DoctorJSON(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	stdout, _, _ := runCmd(t, dir, "doctor", "--json")
	var result struct {
		Checks []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse doctor JSON: %v\nraw: %s", err, stdout)
	}
	if len(result.Checks) == 0 {
		t.Error("expected at least one check")
	}
}

func TestE2E_Log_Empty(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	stdout, _, err := runCmd(t, dir, "log")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if !strings.Contains(stdout, "No history") {
		t.Errorf("expected 'No history' in output, got: %s", stdout)
	}
}

func TestE2E_Log_EmptyJSON(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	stdout, _, err := runCmd(t, dir, "log", "--json")
	if err != nil {
		t.Fatalf("log --json: %v", err)
	}
	var result struct {
		History  []any `json:"history"`
		DMails   []any `json:"dmails"`
		Consumed []any `json:"consumed"`
	}
	parseJSONOutput(t, stdout, &result)
	if len(result.History) != 0 {
		t.Errorf("expected empty history, got %d items", len(result.History))
	}
}

func TestE2E_Sync_Empty(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	stdout, _, err := runCmd(t, dir, "sync")
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	var result struct {
		PendingComments []any `json:"pending_comments"`
	}
	parseJSONOutput(t, stdout, &result)
	if len(result.PendingComments) != 0 {
		t.Errorf("expected no pending comments, got %d", len(result.PendingComments))
	}
}
