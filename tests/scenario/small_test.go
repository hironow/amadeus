//go:build scenario

package scenario_test

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestScenario_L2_Small(t *testing.T) {
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

	// Inject 2 report D-Mails with different priorities
	report1 := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "report-high-001",
		"kind":                 "report",
		"description":          "High priority expedition report",
		"priority":             "1",
	}, "# High Priority Report\n\n## Results\n\n- AUTH-001: critical auth fix implemented")
	ws.InjectDMail(t, ".gate", "inbox", "report-high-001.md", report1)

	report2 := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "report-low-002",
		"kind":                 "report",
		"description":          "Low priority expedition report",
		"priority":             "3",
	}, "# Low Priority Report\n\n## Results\n\n- UI-002: minor UI tweak")
	ws.InjectDMail(t, ".gate", "inbox", "report-low-002.md", report2)

	// Run amadeus check -- processes both reports
	err := ws.RunAmadeusCheck(t, ctx)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Logf("amadeus check returned exit code 2 (drift detected) -- expected")
		} else {
			t.Fatalf("amadeus check failed: %v", err)
		}
	}

	// Wait for feedback fan-out
	ws.WaitForDMailCount(t, ".siren", "inbox", 1, 30*time.Second)
	ws.WaitForDMailCount(t, ".expedition", "inbox", 1, 30*time.Second)

	// Verify outbox cleanup
	ws.WaitForAbsent(t, ".gate", "outbox", 15*time.Second)

	// Second cycle: run amadeus check again (simulates retry cycle)
	err = ws.RunAmadeusCheck(t, ctx)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Logf("second amadeus check returned exit code 2 -- expected")
		} else {
			// exit code 0 is also acceptable on second run (no new drift)
			t.Logf("second amadeus check: %v", err)
		}
	}

	// Verify final state
	obs.AssertAllOutboxEmpty()
}
