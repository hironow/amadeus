//go:build scenario

package scenario_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestScenario_DoDOverrideForceHigh verifies that when the LLM Judge returns
// a DoD fulfillment score >= 70 (the DoDForceHigh threshold), the resulting
// D-Mail severity is escalated to "high" and action to "escalate", regardless
// of the total divergence value.
//
// Mirrors TestScenario_ADROverrideForceHigh but for the DoD axis.
// Unit test: TestDetermineSeverity_DoDOverrideForceHigh in scoring_test.go
func TestScenario_DoDOverrideForceHigh(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "dod_override")
	obs := NewObserver(ws, t)

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	report := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "report-dod-override-001",
		"kind":                 "report",
		"description":          "Expedition report triggering DoD override check",
		"priority":             "1",
	}, "# DoD Override Test Report\n\n## Results\n\n- AUTH-2: 2FA implementation incomplete")
	ws.InjectDMail(t, ".gate", "inbox", "report-dod-override-001.md", report)

	err := ws.RunAmadeusCheck(t, ctx)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Logf("amadeus check returned exit code 2 (drift with DoD override) -- expected")
		} else {
			t.Fatalf("amadeus check failed: %v", err)
		}
	}

	obs.AssertPromptCount(1)

	ws.WaitForDMailCount(t, ".siren", "inbox", 1, 30*time.Second)
	ws.WaitForDMailCount(t, ".expedition", "inbox", 1, 30*time.Second)

	feedbackFiles := ws.ListFiles(t, filepath.Join(ws.RepoPath, ".siren", "inbox"))
	if len(feedbackFiles) == 0 {
		t.Fatal("expected at least 1 feedback D-Mail in .siren/inbox")
	}
	feedbackPath := filepath.Join(ws.RepoPath, ".siren", "inbox", feedbackFiles[0])
	obs.AssertDMailSeverity(feedbackPath, "high")
	obs.AssertDMailAction(feedbackPath, "escalate")

	obs.AssertAllOutboxEmpty()
}
