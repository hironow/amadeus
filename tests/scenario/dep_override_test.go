//go:build scenario

package scenario_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestScenario_DepOverrideForceMedium verifies that when the LLM Judge returns
// a dependency_integrity score >= 80 (DepForceMedium threshold), the resulting
// D-Mail severity is escalated from low to "medium" and action to "retry".
//
// Completes the 3-axis override symmetry:
//   - 010: ADRForceHigh (adr >= 60 -> high)
//   - 022: DoDForceHigh (dod >= 70 -> high)
//   - 026: DepForceMedium (dep >= 80 -> medium) <-- this test
func TestScenario_DepOverrideForceMedium(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "dep_override")
	obs := NewObserver(ws, t)

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	report := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "report-dep-override-001",
		"kind":                 "report",
		"description":          "Expedition report triggering dep override check",
		"priority":             "2",
	}, "# Dep Override Test Report\n\n## Results\n\n- DEP-1: unauthorized dependency added")
	ws.InjectDMail(t, ".gate", "inbox", "report-dep-override-001.md", report)

	err := ws.RunAmadeusCheck(t, ctx)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Logf("amadeus check returned exit code 2 (drift with dep override) -- expected")
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
	obs.AssertDMailSeverity(feedbackPath, "medium")
	obs.AssertDMailAction(feedbackPath, "retry")

	obs.AssertAllOutboxEmpty()
}
