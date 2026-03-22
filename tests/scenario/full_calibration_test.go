//go:build scenario

package scenario_test

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// TestScenario_FullCalibration_ForceFlag verifies the full calibration path
// triggered by the --full flag. This exercises am_full_calibration.json fixture
// (which exists in all fixture levels but is never reached by normal diff-based checks).
//
// The full calibration path differs from diff-based in:
// - Prompt contains "FULL calibration" instead of "Changes Since Last Check"
// - Response includes impact_radius field
// - All source files are evaluated, not just changed ones
func TestScenario_FullCalibration_ForceFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "small")
	obs := NewObserver(ws, t)

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// Inject a report to trigger check
	report := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "report-fullcal-001",
		"kind":                 "report",
		"description":          "Report triggering full calibration check",
		"priority":             "2",
	}, "# Full Calibration Test\n\n## Results\n\n- FEAT-001: feature complete")
	ws.InjectDMail(t, ".gate", "inbox", "report-fullcal-001.md", report)

	// Run amadeus check with --full flag (forces full calibration path)
	err := ws.RunAmadeusCheck(t, ctx, "--full")
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Logf("amadeus check --full returned exit code 2 (drift detected) -- expected")
		} else {
			t.Fatalf("amadeus check --full failed: %v", err)
		}
	}

	// Verify fake-claude was called with full calibration prompt
	obs.AssertPromptCount(1)
	obs.AssertPromptContains([]string{"FULL calibration"})

	// Wait for feedback
	ws.WaitForDMailCount(t, ".siren", "inbox", 1, 30*time.Second)

	obs.AssertAllOutboxEmpty()
}
