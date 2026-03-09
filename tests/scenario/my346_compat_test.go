//go:build scenario

package scenario_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestMY346_LegacyDMailWithLinearIssueID verifies that amadeus check processes
// a D-Mail containing the removed linear_issue_id field without crashing.
// The field is silently dropped by the YAML parser (MY-346 decision).
func TestMY346_LegacyDMailWithLinearIssueID(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "minimal")

	// given: a D-Mail with the removed linear_issue_id field in frontmatter
	legacyDMail := []byte(`---
dmail-schema-version: "1"
name: "legacy-linear-001"
kind: "report"
description: "Report with removed linear_issue_id field"
linear_issue_id: "MY-100"
---

# Legacy Report

This D-Mail uses the removed linear_issue_id field.
It should be silently dropped during parsing.
`)
	ws.InjectDMail(t, ".gate", "inbox", "legacy-linear-001.md", legacyDMail)

	// when: run amadeus check
	err := ws.RunAmadeusCheck(t, ctx)

	// then: should not crash — exit 0 (no drift) or 2 (drift detected) are both acceptable
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Logf("amadeus check returned exit code 2 (drift detected) — expected")
		} else {
			t.Fatalf("amadeus check crashed with legacy linear_issue_id D-Mail: %v", err)
		}
	}

	// Verify the D-Mail was consumed from inbox (moved to archive)
	inboxPath := filepath.Join(ws.RepoPath, ".gate", "inbox", "legacy-linear-001.md")
	if _, statErr := os.Stat(inboxPath); !os.IsNotExist(statErr) {
		t.Error("legacy D-Mail should have been consumed from inbox")
	}
	archivePath := filepath.Join(ws.RepoPath, ".gate", "archive", "legacy-linear-001.md")
	if _, statErr := os.Stat(archivePath); os.IsNotExist(statErr) {
		t.Error("legacy D-Mail should have been archived")
	}
}

// TestMY346_LegacySyncJsonCompositeKey verifies that amadeus sync correctly
// treats D-Mails as pending when sync.json contains legacy single-key entries
// instead of the new composite "dmailName:issueID" format.
func TestMY346_LegacySyncJsonCompositeKey(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "minimal")

	// given: a D-Mail with issues field in the archive
	dmailWithIssues := []byte(`---
dmail-schema-version: "1"
name: "feedback-legacy-key"
kind: "design-feedback"
description: "Feedback with issues"
issues:
  - "MY-42"
---

# Feedback

This D-Mail has issues that should generate composite keys.
`)
	archiveDir := filepath.Join(ws.RepoPath, ".gate", "archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatalf("mkdir archive: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archiveDir, "feedback-legacy-key.md"), dmailWithIssues, 0o644); err != nil {
		t.Fatalf("write archive D-Mail: %v", err)
	}

	// given: sync.json with legacy key format (just dmailName, no :issueID)
	legacySyncState := map[string]any{
		"commented_dmails": map[string]any{
			"feedback-legacy-key": map[string]any{
				"dmail":        "feedback-legacy-key",
				"issue_id":     "",
				"commented_at": "2026-01-01T00:00:00Z",
			},
		},
	}
	syncData, err := json.MarshalIndent(legacySyncState, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy sync state: %v", err)
	}
	syncDir := filepath.Join(ws.RepoPath, ".gate", ".run")
	if err := os.MkdirAll(syncDir, 0o755); err != nil {
		t.Fatalf("mkdir .run: %v", err)
	}
	syncPath := filepath.Join(syncDir, "sync.json")
	if err := os.WriteFile(syncPath, syncData, 0o644); err != nil {
		t.Fatalf("write legacy sync.json: %v", err)
	}

	// when: run amadeus sync
	syncErr := ws.RunAmadeus(t, ctx, "sync", ws.RepoPath)

	// then: should not crash
	if syncErr != nil {
		t.Fatalf("amadeus sync crashed with legacy sync.json: %v", syncErr)
	}

	// Verify the legacy key does NOT match the new composite key format.
	// The D-Mail should appear as pending because "feedback-legacy-key" != "feedback-legacy-key:MY-42".
	updatedSync, readErr := os.ReadFile(syncPath)
	if readErr != nil {
		t.Fatalf("read updated sync.json: %v", readErr)
	}

	// Parse stdout from RunAmadeus would be better, but we can verify sync.json still has legacy entry
	syncContent := string(updatedSync)
	t.Logf("sync.json after amadeus sync: %s", syncContent)

	// Legacy key should still exist (amadeus sync doesn't modify sync.json, only PrintSync reads it)
	var parsedSync map[string]any
	if err := json.Unmarshal(updatedSync, &parsedSync); err != nil {
		t.Fatalf("parse sync.json: %v", err)
	}
	commented, ok := parsedSync["commented_dmails"].(map[string]any)
	if !ok {
		t.Fatal("commented_dmails should be a map")
	}

	// Legacy key exists but composite key does not → D-Mail is pending
	if _, hasLegacy := commented["feedback-legacy-key"]; !hasLegacy {
		t.Error("legacy key should be preserved in sync.json")
	}
	if _, hasComposite := commented["feedback-legacy-key:MY-42"]; hasComposite {
		t.Error("composite key should NOT exist (legacy format doesn't auto-migrate)")
	}
}
