//go:build scenario

package scenario_test

import (
	"context"
	"testing"
	"time"
)

func TestScenario_L1_Minimal(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "minimal")
	obs := NewObserver(ws, t)

	// Start phonewave daemon
	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// Inject a report D-Mail into .gate/inbox (upstream input)
	report := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "report-test-001",
		"kind":                 "report",
		"description":          "Test expedition report",
	}, "# Test Report\n\n## Results\n\n- TEST-001: implemented")
	ws.InjectDMail(t, ".gate", "inbox", "report-test-001.md", report)

	// Start amadeus run as daemon (it watches inbox continuously)
	am := ws.StartAmadeusRun(t, ctx)
	defer ws.StopAmadeusRun(t, am)

	// Wait for amadeus to consume report and produce feedback
	// amadeus run processes inbox D-Mails and writes feedback to outbox
	// phonewave routes: .gate/outbox -> .siren/inbox + .expedition/inbox
	feedbackPath := ws.WaitForDMail(t, ".siren", "inbox", 30*time.Second)
	ws.WaitForDMailCount(t, ".expedition", "inbox", 1, 30*time.Second)

	// Verify outbox is cleaned up
	ws.WaitForAbsent(t, ".gate", "outbox", 10*time.Second)

	// Verify feedback kind
	obs.AssertDMailKind(feedbackPath, "implementation-feedback")

	// Verify closed loop: all 3 delivery points have D-Mails
	obs.WaitForClosedLoop(60 * time.Second)

	obs.AssertAllOutboxEmpty()
}
