//go:build scenario

package scenario_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestScenario_ApproveCmdPath(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "minimal")
	obs := NewObserver(ws, t)

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// Inject a report D-Mail for amadeus to consume
	reportContent := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "test-report",
		"kind":                 "report",
		"description":          "Test report for approve-cmd",
	}, "# Test Report\n\nThis is a test report for the approve-cmd scenario test.")
	ws.InjectDMail(t, ".gate", "inbox", "test-report.md", reportContent)

	// Create approve script (exit 0 = approve all)
	approveScript := filepath.Join(ws.Root, "approve.sh")
	if err := os.WriteFile(approveScript, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write approve script: %v", err)
	}

	// Create notify script that logs invocations for verification
	notifyLog := filepath.Join(ws.Root, "notify.log")
	notifyScript := filepath.Join(ws.Root, "notify.sh")
	notifyContent := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %s\n", notifyLog)
	if err := os.WriteFile(notifyScript, []byte(notifyContent), 0o755); err != nil {
		t.Fatalf("write notify script: %v", err)
	}

	// Run amadeus with --approve-cmd and --notify-cmd (NOT --auto-approve)
	err := ws.RunAmadeus(t, ctx, "check",
		"--approve-cmd", approveScript,
		"--notify-cmd", notifyScript,
		ws.RepoPath,
	)
	if err != nil {
		// Exit code 2 = drift detected, which is expected
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Logf("amadeus check returned exit code 2 (drift detected) — expected")
		} else {
			t.Fatalf("amadeus check with approve-cmd failed: %v", err)
		}
	}

	// Verify feedback was produced and delivered to .siren/inbox
	feedbackPath := ws.WaitForDMail(t, ".siren", "inbox", 30*time.Second)
	obs.AssertDMailKind(feedbackPath, "design-feedback")

	// Verify outbox was flushed
	ws.WaitForAbsent(t, ".gate", "outbox", 10*time.Second)

	// Verify notify script was invoked (amadeus notifies on check completion)
	data, err := os.ReadFile(notifyLog)
	if err != nil {
		t.Fatalf("notify.log not found — notify-cmd was not invoked: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("notify.log exists but is empty — notify-cmd produced no output")
	}
	t.Logf("notify.log content:\n%s", string(data))
}
