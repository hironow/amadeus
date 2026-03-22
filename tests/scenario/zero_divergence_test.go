//go:build scenario

package scenario_test

import (
	"context"
	"testing"
	"time"
)

// TestScenario_ZeroDivergence_NoDMailGenerated verifies the clean pass path:
// when all divergence axes score zero and dmails=[], amadeus should NOT generate
// any feedback D-Mails. This tests the "nothing happened" base case.
func TestScenario_ZeroDivergence_NoDMailGenerated(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "zero")
	obs := NewObserver(ws, t)

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	report := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "report-zero-001",
		"kind":                 "report",
		"description":          "Report triggering zero-divergence check",
		"priority":             "3",
	}, "# Zero Divergence Test\n\n## Results\n\n- FEAT-001: clean implementation")
	ws.InjectDMail(t, ".gate", "inbox", "report-zero-001.md", report)

	// Run amadeus check — exit code 0 expected (no drift)
	err := ws.RunAmadeusCheck(t, ctx)
	if err != nil {
		t.Fatalf("amadeus check failed: %v (exit 0 expected for zero divergence)", err)
	}

	obs.AssertPromptCount(1)

	// Key assertion: no feedback D-Mails should be generated
	// Poll for outbox to be empty instead of sleeping
	ws.WaitForAbsent(t, ".gate", "outbox", 10*time.Second)
	obs.AssertAllOutboxEmpty()
	obs.AssertDMailCount(".siren", "inbox", 0)
	obs.AssertDMailCount(".expedition", "inbox", 0)
}
