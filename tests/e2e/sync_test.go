//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

func TestE2E_Sync_WithPendingComments(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	// Seed D-Mails with issues
	seedDMails(t, dir, []seedDMailSpec{
		{Name: "feedback-001", Kind: "feedback", Description: "ADR issue", Severity: "low", Issues: []string{"MY-100"}},
		{Name: "feedback-002", Kind: "feedback", Description: "DoD issue", Severity: "low", Issues: []string{"MY-200", "MY-300"}},
	})

	stdout, _, err := runCmd(t, dir, "sync")
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	var result struct {
		PendingComments []struct {
			DMail       string `json:"dmail"`
			IssueID     string `json:"issue_id"`
			Status      string `json:"status"`
			Description string `json:"description"`
		} `json:"pending_comments"`
	}
	parseJSONOutput(t, stdout, &result)

	// 3 pending comments: feedback-001:MY-100, feedback-002:MY-200, feedback-002:MY-300
	if len(result.PendingComments) != 3 {
		t.Fatalf("expected 3 pending comments, got %d", len(result.PendingComments))
	}

	// Verify first
	found := map[string]bool{}
	for _, pc := range result.PendingComments {
		key := pc.DMail + ":" + pc.IssueID
		found[key] = true
	}
	for _, expected := range []string{"feedback-001:MY-100", "feedback-002:MY-200", "feedback-002:MY-300"} {
		if !found[expected] {
			t.Errorf("missing pending comment: %s", expected)
		}
	}
}

func TestE2E_Sync_NoIssues_NoPending(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	// Seed D-Mail without issues
	seedDMails(t, dir, []seedDMailSpec{
		{Name: "feedback-001", Kind: "feedback", Description: "No issues", Severity: "low"},
	})

	stdout, _, err := runCmd(t, dir, "sync")
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	var result struct {
		PendingComments []any `json:"pending_comments"`
	}
	parseJSONOutput(t, stdout, &result)

	if len(result.PendingComments) != 0 {
		t.Errorf("expected 0 pending comments for D-Mail without issues, got %d", len(result.PendingComments))
	}
}

func TestE2E_MarkCommented_Text(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	// Seed a D-Mail so .gate/ is valid
	seedDMails(t, dir, []seedDMailSpec{
		{Name: "feedback-001", Kind: "feedback", Description: "Test", Severity: "low", Issues: []string{"MY-100"}},
	})

	_, stderr, err := runCmd(t, dir, "mark-commented", "feedback-001", "MY-100")
	if err != nil {
		t.Fatalf("mark-commented: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stderr, "feedback-001:MY-100") {
		t.Errorf("expected 'feedback-001:MY-100' in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "commented") {
		t.Errorf("expected 'commented' in stderr, got: %s", stderr)
	}
}

func TestE2E_MarkCommented_JSON(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	seedDMails(t, dir, []seedDMailSpec{
		{Name: "feedback-001", Kind: "feedback", Description: "Test", Severity: "low", Issues: []string{"MY-100"}},
	})

	stdout, _, err := runCmd(t, dir, "mark-commented", "feedback-001", "MY-100", "--json")
	if err != nil {
		t.Fatalf("mark-commented --json: %v", err)
	}

	var result struct {
		DMail   string `json:"dmail"`
		IssueID string `json:"issue_id"`
		Status  string `json:"status"`
	}
	parseJSONOutput(t, stdout, &result)

	if result.DMail != "feedback-001" {
		t.Errorf("expected dmail=feedback-001, got %s", result.DMail)
	}
	if result.IssueID != "MY-100" {
		t.Errorf("expected issue_id=MY-100, got %s", result.IssueID)
	}
	if result.Status != "commented" {
		t.Errorf("expected status=commented, got %s", result.Status)
	}
}

func TestE2E_MarkCommented_RemovesFromSync(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	seedDMails(t, dir, []seedDMailSpec{
		{Name: "feedback-001", Kind: "feedback", Description: "Test", Severity: "low", Issues: []string{"MY-100", "MY-200"}},
	})

	// Before: 2 pending comments
	stdout, _, _ := runCmd(t, dir, "sync")
	var before struct {
		PendingComments []any `json:"pending_comments"`
	}
	parseJSONOutput(t, stdout, &before)
	if len(before.PendingComments) != 2 {
		t.Fatalf("expected 2 pending comments before, got %d", len(before.PendingComments))
	}

	// Mark one as commented
	runCmd(t, dir, "mark-commented", "feedback-001", "MY-100")

	// After: 1 pending comment
	stdout, _, _ = runCmd(t, dir, "sync")
	var after struct {
		PendingComments []struct {
			DMail   string `json:"dmail"`
			IssueID string `json:"issue_id"`
		} `json:"pending_comments"`
	}
	parseJSONOutput(t, stdout, &after)
	if len(after.PendingComments) != 1 {
		t.Fatalf("expected 1 pending comment after marking, got %d", len(after.PendingComments))
	}
	if after.PendingComments[0].IssueID != "MY-200" {
		t.Errorf("expected remaining pending for MY-200, got %s", after.PendingComments[0].IssueID)
	}
}

func TestE2E_MarkCommented_AllMarked_EmptySync(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	seedDMails(t, dir, []seedDMailSpec{
		{Name: "feedback-001", Kind: "feedback", Description: "Test", Severity: "low", Issues: []string{"MY-100"}},
	})

	// Mark the only comment
	runCmd(t, dir, "mark-commented", "feedback-001", "MY-100")

	// Sync should show empty
	stdout, _, _ := runCmd(t, dir, "sync")
	var result struct {
		PendingComments []any `json:"pending_comments"`
	}
	parseJSONOutput(t, stdout, &result)
	if len(result.PendingComments) != 0 {
		t.Errorf("expected 0 pending after marking all, got %d", len(result.PendingComments))
	}
}

func TestE2E_MarkCommented_ErrorMissingArgs(t *testing.T) {
	dir := initTestRepo(t)
	_, _, err := runCmd(t, dir, "mark-commented")
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestE2E_MarkCommented_ErrorTooManyArgs(t *testing.T) {
	dir := initTestRepo(t)
	_, _, err := runCmd(t, dir, "mark-commented", "a", "b", "c")
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}
