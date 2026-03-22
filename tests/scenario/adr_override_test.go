//go:build scenario

package scenario_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestScenario_ADROverrideForceHigh verifies that when the LLM Judge returns
// an ADR integrity score >= 60 (the ADRForceHigh threshold), the resulting
// D-Mail severity is escalated to "high" and action to "escalate", regardless
// of the total divergence value.
//
// This tests the CalcDivergence -> DetermineSeverity -> D-Mail frontmatter
// pipeline end-to-end. The unit test TestDetermineSeverity_ADROverrideForceHigh
// covers the pure function; this scenario test covers the full flow including
// fake-claude fixture -> amadeus check -> D-Mail generation -> phonewave delivery.
func TestScenario_ADROverrideForceHigh(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Use the adr_override fixture level where am_check.json has ADR score=65
	ws := NewWorkspace(t, "adr_override")
	obs := NewObserver(ws, t)

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// Inject a report that triggers amadeus check
	report := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "report-adr-override-001",
		"kind":                 "report",
		"description":          "Expedition report triggering ADR override check",
		"priority":             "1",
	}, "# ADR Override Test Report\n\n## Results\n\n- AUTH-3: JWT refresh token changes")
	ws.InjectDMail(t, ".gate", "inbox", "report-adr-override-001.md", report)

	// Run amadeus check -- should detect ADR override
	err := ws.RunAmadeusCheck(t, ctx)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Logf("amadeus check returned exit code 2 (drift detected with ADR override) -- expected")
		} else {
			t.Fatalf("amadeus check failed: %v", err)
		}
	}

	// Verify fake-claude was actually called (not real API)
	obs.AssertPromptCount(1)

	// Wait for feedback fan-out
	ws.WaitForDMailCount(t, ".siren", "inbox", 1, 30*time.Second)
	ws.WaitForDMailCount(t, ".expedition", "inbox", 1, 30*time.Second)

	// Verify the feedback D-Mail has escalated severity and action
	feedbackFiles := ws.ListFiles(t, filepath.Join(ws.RepoPath, ".siren", "inbox"))
	if len(feedbackFiles) == 0 {
		t.Fatal("expected at least 1 feedback D-Mail in .siren/inbox")
	}
	feedbackPath := filepath.Join(ws.RepoPath, ".siren", "inbox", feedbackFiles[0])
	obs.AssertDMailSeverity(feedbackPath, "high")
	obs.AssertDMailAction(feedbackPath, "escalate")

	// Verify outbox cleanup
	obs.AssertAllOutboxEmpty()
}
