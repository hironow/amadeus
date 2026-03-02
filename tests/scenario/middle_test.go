//go:build scenario

package scenario_test

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestScenario_L3_Middle(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "middle")
	obs := NewObserver(ws, t)

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// Inject 3 report D-Mails
	for i, spec := range []struct {
		name     string
		priority string
		desc     string
		body     string
	}{
		{"report-critical-001", "1", "Critical auth report", "# Critical\n\n- AUTH-001: critical fix"},
		{"report-medium-002", "2", "Medium priority report", "# Medium\n\n- DATA-002: data layer fix"},
		{"report-low-003", "3", "Low priority report", "# Low\n\n- UI-003: minor styling"},
	} {
		report := FormatDMail(map[string]string{
			"dmail-schema-version": "1",
			"name":                 spec.name,
			"kind":                 "report",
			"description":          spec.desc,
			"priority":             spec.priority,
		}, spec.body)
		ws.InjectDMail(t, ".gate", "inbox", spec.name+".md", report)
		t.Logf("injected report %d: %s (priority %s)", i+1, spec.name, spec.priority)
	}

	// First amadeus check
	err := ws.RunAmadeusCheck(t, ctx)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Logf("first check: exit code 2 (drift detected) — expected")
		} else {
			t.Fatalf("first amadeus check failed: %v", err)
		}
	}

	// Wait for feedback delivery
	ws.WaitForDMailCount(t, ".siren", "inbox", 1, 30*time.Second)
	ws.WaitForDMailCount(t, ".expedition", "inbox", 1, 30*time.Second)
	ws.WaitForAbsent(t, ".gate", "outbox", 15*time.Second)

	// Inject convergence D-Mail (simulates upstream convergence signal)
	convergence := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "convergence-001",
		"kind":                 "convergence",
		"description":          "System convergence checkpoint",
	}, "# Convergence\n\nAll tools have stabilized.")
	ws.InjectDMail(t, ".gate", "inbox", "convergence-001.md", convergence)

	// Second amadeus check (consecutive)
	err = ws.RunAmadeusCheck(t, ctx)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Logf("second check: exit code 2 — expected")
		} else {
			t.Logf("second check result: %v (may be 0 if no new drift)", err)
		}
	}

	// Verify no deadlock, all outboxes eventually empty
	ws.WaitForAbsent(t, ".gate", "outbox", 15*time.Second)
	obs.AssertAllOutboxEmpty()
}
