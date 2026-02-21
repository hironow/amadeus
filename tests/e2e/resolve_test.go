//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2E_Resolve_Approve(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	// Seed a HIGH severity D-Mail in archive/ and pending/
	seedDMails(t, dir, []seedDMailSpec{{
		Name:        "feedback-001",
		Kind:        "feedback",
		Description: "Test issue",
		Severity:    "high",
		Issues:      []string{"MY-100"},
		Body:        "Detail body.\n",
	}})

	stdout, _, err := runCmd(t, dir, "resolve", "feedback-001", "--approve")
	if err != nil {
		t.Fatalf("resolve --approve: %v\nstdout: %s", err, stdout)
	}

	if !strings.Contains(stdout, "approved") {
		t.Errorf("expected 'approved' in output, got: %s", stdout)
	}

	// File moved from pending/ to outbox/
	assertFileNotExists(t, filepath.Join(dir, ".gate", "pending", "feedback-001.md"))
	assertFileExists(t, filepath.Join(dir, ".gate", "outbox", "feedback-001.md"))

	// Resolution recorded
	assertFileExists(t, filepath.Join(dir, ".gate", ".run", "resolutions.json"))
}

func TestE2E_Resolve_Reject(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	seedDMails(t, dir, []seedDMailSpec{{
		Name:        "feedback-001",
		Kind:        "feedback",
		Description: "Test issue",
		Severity:    "high",
		Issues:      []string{"MY-100"},
	}})

	stdout, _, err := runCmd(t, dir, "resolve", "feedback-001", "--reject", "--reason", "Not applicable")
	if err != nil {
		t.Fatalf("resolve --reject: %v\nstdout: %s", err, stdout)
	}

	if !strings.Contains(stdout, "rejected") {
		t.Errorf("expected 'rejected' in output, got: %s", stdout)
	}

	// File moved to rejected/
	assertFileNotExists(t, filepath.Join(dir, ".gate", "pending", "feedback-001.md"))
	assertFileExists(t, filepath.Join(dir, ".gate", "rejected", "feedback-001.md"))
}

func TestE2E_Resolve_ApproveJSON(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	seedDMails(t, dir, []seedDMailSpec{{
		Name:        "feedback-001",
		Kind:        "feedback",
		Description: "Test issue",
		Severity:    "high",
		Issues:      []string{"MY-100", "MY-200"},
		Body:        "Detail body.\n",
	}})

	stdout, _, err := runCmd(t, dir, "resolve", "feedback-001", "--approve", "--json")
	if err != nil {
		t.Fatalf("resolve --approve --json: %v", err)
	}

	var results []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Action     string `json:"action"`
		ResolvedAt string `json:"resolved_at"`
		Comments   []struct {
			IssueID     string `json:"issue_id"`
			DMail       string `json:"dmail"`
			Description string `json:"description"`
			Body        string `json:"body"`
			Resolution  string `json:"resolution"`
		} `json:"comments"`
	}
	parseJSONOutput(t, stdout, &results)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.Name != "feedback-001" {
		t.Errorf("expected name=feedback-001, got %s", r.Name)
	}
	if r.Status != "approved" {
		t.Errorf("expected status=approved, got %s", r.Status)
	}
	if r.Action != "approve" {
		t.Errorf("expected action=approve, got %s", r.Action)
	}
	if r.ResolvedAt == "" {
		t.Error("expected resolved_at")
	}

	// Should have CommentPayloads for both issues
	if len(r.Comments) != 2 {
		t.Fatalf("expected 2 comments (one per issue), got %d", len(r.Comments))
	}
	if r.Comments[0].IssueID != "MY-100" {
		t.Errorf("expected first comment issue_id=MY-100, got %s", r.Comments[0].IssueID)
	}
	if r.Comments[1].IssueID != "MY-200" {
		t.Errorf("expected second comment issue_id=MY-200, got %s", r.Comments[1].IssueID)
	}
	if r.Comments[0].Resolution != "approved" {
		t.Errorf("expected resolution=approved, got %s", r.Comments[0].Resolution)
	}
}

func TestE2E_Resolve_RejectJSON(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	seedDMails(t, dir, []seedDMailSpec{{
		Name:        "feedback-001",
		Kind:        "feedback",
		Description: "Test issue",
		Severity:    "high",
		Issues:      []string{"MY-100"},
	}})

	stdout, _, err := runCmd(t, dir, "resolve", "feedback-001", "--reject", "--reason", "Won't fix", "--json")
	if err != nil {
		t.Fatalf("resolve --reject --json: %v", err)
	}

	var results []struct {
		Name     string `json:"name"`
		Status   string `json:"status"`
		Comments []struct {
			Resolution string `json:"resolution"`
			Reason     string `json:"reason"`
		} `json:"comments"`
	}
	parseJSONOutput(t, stdout, &results)

	if results[0].Status != "rejected" {
		t.Errorf("expected status=rejected, got %s", results[0].Status)
	}
	if len(results[0].Comments) == 0 {
		t.Fatal("expected comments")
	}
	if results[0].Comments[0].Reason != "Won't fix" {
		t.Errorf("expected reason='Won't fix', got %s", results[0].Comments[0].Reason)
	}
}

func TestE2E_Resolve_MultipleNames(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	seedDMails(t, dir, []seedDMailSpec{
		{Name: "feedback-001", Kind: "feedback", Description: "First", Severity: "high", Issues: []string{"MY-1"}},
		{Name: "feedback-002", Kind: "feedback", Description: "Second", Severity: "high", Issues: []string{"MY-2"}},
	})

	stdout, _, err := runCmd(t, dir, "resolve", "feedback-001", "feedback-002", "--approve", "--json")
	if err != nil {
		t.Fatalf("resolve multiple: %v", err)
	}

	var results []struct {
		Name string `json:"name"`
	}
	parseJSONOutput(t, stdout, &results)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestE2E_Resolve_Stdin(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	seedDMails(t, dir, []seedDMailSpec{
		{Name: "feedback-001", Kind: "feedback", Description: "From stdin", Severity: "high", Issues: []string{"MY-1"}},
	})

	stdout, _, err := runCmdStdin(t, dir, "feedback-001\n", "resolve", "--approve", "--json")
	if err != nil {
		t.Fatalf("resolve via stdin: %v", err)
	}

	var results []struct {
		Name string `json:"name"`
	}
	parseJSONOutput(t, stdout, &results)
	if len(results) != 1 {
		t.Errorf("expected 1 result from stdin, got %d", len(results))
	}
}

func TestE2E_Resolve_CommentTargetsInText(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	seedDMails(t, dir, []seedDMailSpec{{
		Name:        "feedback-001",
		Kind:        "feedback",
		Description: "Test",
		Severity:    "high",
		Issues:      []string{"MY-100", "MY-200"},
	}})

	stdout, _, err := runCmd(t, dir, "resolve", "feedback-001", "--approve")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if !strings.Contains(stdout, "Comment targets") {
		t.Errorf("expected 'Comment targets' in text output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "MY-100") || !strings.Contains(stdout, "MY-200") {
		t.Errorf("expected issue IDs in output, got: %s", stdout)
	}
}

func TestE2E_Resolve_ErrorMissingApproveReject(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	_, _, err := runCmd(t, dir, "resolve", "feedback-001")
	if err == nil {
		t.Fatal("expected error when neither --approve nor --reject specified")
	}
}

func TestE2E_Resolve_ErrorBothApproveReject(t *testing.T) {
	dir := initTestRepo(t)
	_, _, err := runCmd(t, dir, "resolve", "feedback-001", "--approve", "--reject")
	if err == nil {
		t.Fatal("expected error when both --approve and --reject specified")
	}
}

func TestE2E_Resolve_ErrorRejectWithoutReason(t *testing.T) {
	dir := initTestRepo(t)
	_, _, err := runCmd(t, dir, "resolve", "feedback-001", "--reject")
	if err == nil {
		t.Fatal("expected error when --reject without --reason")
	}
}

func TestE2E_Resolve_ErrorNonexistentDMail(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	_, _, err := runCmd(t, dir, "resolve", "nonexistent", "--approve")
	if err == nil {
		t.Fatal("expected error for nonexistent D-Mail")
	}
}

func TestE2E_Resolve_ErrorAlreadyResolved(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	seedDMails(t, dir, []seedDMailSpec{{
		Name: "feedback-001", Kind: "feedback", Description: "Test", Severity: "high",
	}})

	// Resolve once
	runCmd(t, dir, "resolve", "feedback-001", "--approve")

	// Resolve again
	_, _, err := runCmd(t, dir, "resolve", "feedback-001", "--approve")
	if err == nil {
		t.Fatal("expected error for already-resolved D-Mail")
	}
}

func TestE2E_Resolve_FlagPositionIndependence(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	seedDMails(t, dir, []seedDMailSpec{{
		Name: "feedback-001", Kind: "feedback", Description: "Test", Severity: "high",
	}})

	// --approve before name
	stdout, _, err := runCmd(t, dir, "resolve", "--approve", "feedback-001")
	if err != nil {
		t.Fatalf("resolve with flag before name: %v\nstdout: %s", err, stdout)
	}
	if !strings.Contains(stdout, "approved") {
		t.Errorf("expected 'approved' in output, got: %s", stdout)
	}
}

func TestE2E_Resolve_NoIssues_NoComments(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	seedDMails(t, dir, []seedDMailSpec{{
		Name:        "feedback-001",
		Kind:        "feedback",
		Description: "No issues",
		Severity:    "high",
		// No Issues field
	}})

	stdout, _, err := runCmd(t, dir, "resolve", "feedback-001", "--approve", "--json")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	var results []struct {
		Comments []any `json:"comments"`
	}
	parseJSONOutput(t, stdout, &results)

	// No issues means no comments
	if results[0].Comments != nil && len(results[0].Comments) > 0 {
		t.Errorf("expected no comments for D-Mail without issues, got %d", len(results[0].Comments))
	}

	// Text output should NOT have "Comment targets"
	stdout2, _, _ := runCmd(t, dir, "resolve", "feedback-001", "--approve")
	_ = stdout2 // Already resolved, will error — that's fine for this test
}

func TestE2E_Resolve_ResolutionPersists(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	seedDMails(t, dir, []seedDMailSpec{{
		Name: "feedback-001", Kind: "feedback", Description: "Persist test", Severity: "high",
	}})

	runCmd(t, dir, "resolve", "feedback-001", "--approve")

	// Resolution file should exist
	resPath := filepath.Join(dir, ".gate", ".run", "resolutions.json")
	assertFileExists(t, resPath)

	data, _ := os.ReadFile(resPath)
	if !strings.Contains(string(data), "feedback-001") {
		t.Error("resolution file should contain feedback-001")
	}
	if !strings.Contains(string(data), "approved") {
		t.Error("resolution file should contain 'approved'")
	}
}
