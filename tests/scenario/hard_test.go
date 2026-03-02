//go:build scenario

package scenario_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestScenario_L4_Hard(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "hard")
	obs := NewObserver(ws, t)

	// --- Phase 1: phonewave daemon restart ---
	pw := ws.StartPhonewave(t, ctx)
	defer ws.DumpPhonewaveLog(t, pw)

	// Inject report before restart
	report1 := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "report-pre-restart",
		"kind":                 "report",
		"description":          "Report before daemon restart",
	}, "# Pre-Restart Report\n\n- RESTART-001: test")
	ws.InjectDMail(t, ".gate", "inbox", "report-pre-restart.md", report1)

	// Run amadeus (may or may not succeed depending on timing)
	err := ws.RunAmadeusCheck(t, ctx)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Logf("pre-restart check: exit code 2 — expected")
		} else {
			t.Logf("pre-restart check: %v (acceptable during restart test)", err)
		}
	}

	// Restart phonewave daemon
	t.Log("restarting phonewave daemon")
	ws.StopPhonewave(t, pw)
	time.Sleep(1 * time.Second) // Brief pause for cleanup
	pw = ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)

	// Wait for any pending deliveries to complete after restart
	ws.WaitForAbsent(t, ".gate", "outbox", 30*time.Second)

	// --- Phase 2: fake-claude transient failure ---
	// Set FAKE_CLAUDE_FAIL_COUNT=2: fake-claude fails 2 times, succeeds on 3rd call
	// The counter file must be reset between phases
	counterPath := filepath.Join(os.TempDir(), "fake-claude-call-count")
	os.Remove(counterPath) // Reset counter
	ws.Env = append(ws.Env, "FAKE_CLAUDE_FAIL_COUNT=2")

	report2 := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "report-with-failures",
		"kind":                 "report",
		"description":          "Report triggering transient failures",
	}, "# Transient Failure Report\n\n- FAIL-001: test failure recovery")
	ws.InjectDMail(t, ".gate", "inbox", "report-with-failures.md", report2)

	// First two amadeus checks will have fake-claude fail
	for i := 0; i < 2; i++ {
		err := ws.RunAmadeusCheck(t, ctx)
		if err != nil {
			t.Logf("check %d with FAIL_COUNT: %v (expected failure)", i+1, err)
		}
	}

	// Third check should succeed (counter exceeded FAIL_COUNT)
	err = ws.RunAmadeusCheck(t, ctx)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Logf("recovery check: exit code 2 (drift detected) — expected")
		} else {
			t.Fatalf("recovery check failed unexpectedly: %v", err)
		}
	}

	// Clean up FAIL_COUNT env
	cleanEnv := make([]string, 0, len(ws.Env))
	for _, e := range ws.Env {
		if e != "FAKE_CLAUDE_FAIL_COUNT=2" {
			cleanEnv = append(cleanEnv, e)
		}
	}
	ws.Env = cleanEnv
	os.Remove(counterPath)

	// --- Phase 3: malformed D-Mail ---
	// Inject a malformed D-Mail (no valid frontmatter)
	malformed := []byte("This is not a valid D-Mail.\nNo YAML frontmatter here.\n")
	ws.InjectDMail(t, ".gate", "inbox", "malformed-001.md", malformed)

	// The system should handle this gracefully (skip or error-queue)
	// Run a final check — the malformed D-Mail should not crash anything
	err = ws.RunAmadeusCheck(t, ctx)
	if err != nil {
		t.Logf("check after malformed inject: %v (acceptable)", err)
	}

	// Wait for system to stabilize
	time.Sleep(3 * time.Second)

	// --- Final verification ---
	// The system should have recovered: outboxes eventually empty
	ws.WaitForAbsent(t, ".gate", "outbox", 30*time.Second)
	obs.AssertAllOutboxEmpty()
	t.Log("L4 hard test passed: daemon restart + transient failures + malformed D-Mail all handled")
}
