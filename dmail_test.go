package amadeus

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseDMail_Valid(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: feedback
description: "ADR-003 violation detected"
issues:
  - MY-42
severity: high
metadata:
  created_at: "2026-02-20T12:00:00Z"
---

# ADR-003 Violation

The auth module violates the JWT requirement.
`
	dmail, err := ParseDMail([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}
	if dmail.Name != "feedback-001" {
		t.Errorf("expected name feedback-001, got %s", dmail.Name)
	}
	if dmail.Kind != KindFeedback {
		t.Errorf("expected kind feedback, got %s", dmail.Kind)
	}
	if dmail.Description != "ADR-003 violation detected" {
		t.Errorf("expected description, got %s", dmail.Description)
	}
	if len(dmail.Issues) != 1 || dmail.Issues[0] != "MY-42" {
		t.Errorf("expected issues [MY-42], got %v", dmail.Issues)
	}
	if dmail.Severity != SeverityHigh {
		t.Errorf("expected severity high, got %s", dmail.Severity)
	}
	if dmail.Metadata["created_at"] != "2026-02-20T12:00:00Z" {
		t.Errorf("expected metadata created_at, got %v", dmail.Metadata)
	}
	if !strings.Contains(dmail.Body, "ADR-003 Violation") {
		t.Errorf("expected body to contain 'ADR-003 Violation', got %s", dmail.Body)
	}
}

func TestParseDMail_Minimal(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: feedback
description: "minimal"
---
`
	dmail, err := ParseDMail([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}
	if dmail.Name != "feedback-001" {
		t.Errorf("expected name feedback-001, got %s", dmail.Name)
	}
	if dmail.Kind != KindFeedback {
		t.Errorf("expected kind feedback, got %s", dmail.Kind)
	}
	if len(dmail.Issues) != 0 {
		t.Errorf("expected empty issues, got %v", dmail.Issues)
	}
	if dmail.Severity != "" {
		t.Errorf("expected empty severity, got %s", dmail.Severity)
	}
}

func TestParseDMail_InvalidYAML(t *testing.T) {
	raw := `---
name: [invalid
---
`
	_, err := ParseDMail([]byte(raw))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParseDMail_MissingDelimiters(t *testing.T) {
	raw := `no frontmatter here`
	_, err := ParseDMail([]byte(raw))
	if err == nil {
		t.Error("expected error for missing delimiters")
	}
}

func TestParseDMail_LegacyUppercaseSeverity(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: feedback
description: "legacy uppercase severity"
severity: HIGH
---
`
	dmail, err := ParseDMail([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}
	if dmail.Severity != SeverityHigh {
		t.Errorf("expected severity 'high', got %q", dmail.Severity)
	}
}

func TestParseDMail_LegacyMixedCaseSeverity(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: feedback
description: "mixed case"
severity: Medium
---
`
	dmail, err := ParseDMail([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}
	if dmail.Severity != SeverityMedium {
		t.Errorf("expected severity 'medium', got %q", dmail.Severity)
	}
}

func TestMarshalDMail_RoundTrip(t *testing.T) {
	original := DMail{
		Name:        "feedback-001",
		Kind:        KindFeedback,
		Description: "ADR violation",
		Issues:      []string{"MY-42"},
		Severity:    SeverityHigh,
		Metadata:    map[string]string{"created_at": "2026-02-20T12:00:00Z"},
		Body:        "# Details\n\nSome markdown content.\n",
	}

	data, err := MarshalDMail(original)
	if err != nil {
		t.Fatalf("MarshalDMail failed: %v", err)
	}

	// then: raw content starts with ---
	if !strings.HasPrefix(string(data), "---\n") {
		t.Errorf("expected data to start with '---', got: %s", string(data[:20]))
	}

	// round-trip
	parsed, err := ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail round-trip failed: %v", err)
	}
	if parsed.Name != original.Name {
		t.Errorf("expected name %s, got %s", original.Name, parsed.Name)
	}
	if parsed.Kind != original.Kind {
		t.Errorf("expected kind %s, got %s", original.Kind, parsed.Kind)
	}
	if parsed.Description != original.Description {
		t.Errorf("expected description %s, got %s", original.Description, parsed.Description)
	}
	if len(parsed.Issues) != 1 || parsed.Issues[0] != "MY-42" {
		t.Errorf("expected issues [MY-42], got %v", parsed.Issues)
	}
	if parsed.Severity != original.Severity {
		t.Errorf("expected severity %s, got %s", original.Severity, parsed.Severity)
	}
	if !strings.Contains(parsed.Body, "Some markdown content.") {
		t.Errorf("expected body to contain 'Some markdown content.', got %s", parsed.Body)
	}
}

func TestInitGateDir_CreatesNewStructure(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"outbox", "inbox", "archive", "pending", "rejected"} {
		path := filepath.Join(root, sub)
		info, err := statDir(path)
		if err != nil {
			t.Errorf("expected %s to exist: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", sub)
		}
	}
}

func TestInitGateDir_GitignoreContainsOutboxInbox(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	data, err := readGitignore(root)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "outbox/") {
		t.Errorf("expected .gitignore to contain 'outbox/', got: %s", content)
	}
	if !strings.Contains(content, "inbox/") {
		t.Errorf("expected .gitignore to contain 'inbox/', got: %s", content)
	}
	if !strings.Contains(content, "pending/") {
		t.Errorf("expected .gitignore to contain 'pending/', got: %s", content)
	}
	if !strings.Contains(content, "rejected/") {
		t.Errorf("expected .gitignore to contain 'rejected/', got: %s", content)
	}
}

func TestMovePendingToRejected_CreatesDirectoryOnDemand(t *testing.T) {
	// given: a repo where rejected/ does not exist (pre-update installation)
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	dmail := DMail{
		Name:     "feedback-001",
		Kind:     KindFeedback,
		Severity: SeverityHigh,
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}
	// Remove rejected/ to simulate pre-update repo
	if err := os.Remove(filepath.Join(root, "rejected")); err != nil {
		t.Fatal(err)
	}

	// when: move to rejected
	err := store.MovePendingToRejected("feedback-001")

	// then: should succeed (directory created on demand)
	if err != nil {
		t.Fatalf("MovePendingToRejected failed on missing dir: %v", err)
	}
	rejectedPath := filepath.Join(root, "rejected", "feedback-001.md")
	if _, statErr := os.Stat(rejectedPath); statErr != nil {
		t.Errorf("expected file in rejected/: %v", statErr)
	}
}

func TestNextDMailName_EmptyArchive(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	name, err := store.NextDMailName(KindFeedback)
	if err != nil {
		t.Fatal(err)
	}
	if name != "feedback-001" {
		t.Errorf("expected feedback-001, got %s", name)
	}
}

func TestNextDMailName_Sequential(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	dmail := DMail{
		Name:        "feedback-001",
		Kind:        KindFeedback,
		Description: "first",
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}

	name, err := store.NextDMailName(KindFeedback)
	if err != nil {
		t.Fatal(err)
	}
	if name != "feedback-002" {
		t.Errorf("expected feedback-002, got %s", name)
	}
}

func TestSaveDMail_DualWrite(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	dmail := DMail{
		Name:        "feedback-001",
		Kind:        KindFeedback,
		Description: "dual write test",
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}

	// then: file exists in both outbox/ and archive/
	outboxPath := filepath.Join(root, "outbox", "feedback-001.md")
	archivePath := filepath.Join(root, "archive", "feedback-001.md")
	if _, err := statDir(outboxPath); err != nil {
		t.Errorf("expected file in outbox: %v", err)
	}
	if _, err := statDir(archivePath); err != nil {
		t.Errorf("expected file in archive: %v", err)
	}
}

func TestSaveDMail_HighSeverity_WritesToPending(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	dmail := DMail{
		Name:        "feedback-001",
		Kind:        KindFeedback,
		Description: "high severity test",
		Severity:    SeverityHigh,
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}

	// then: file exists in archive/ and pending/ (NOT outbox/)
	archivePath := filepath.Join(root, "archive", "feedback-001.md")
	pendingPath := filepath.Join(root, "pending", "feedback-001.md")
	outboxPath := filepath.Join(root, "outbox", "feedback-001.md")
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("expected file in archive: %v", err)
	}
	if _, err := os.Stat(pendingPath); err != nil {
		t.Errorf("expected file in pending: %v", err)
	}
	if _, err := os.Stat(outboxPath); err == nil {
		t.Error("expected file NOT in outbox for HIGH severity")
	}
}

func TestSaveDMail_LowSeverity_WritesToOutbox(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	dmail := DMail{
		Name:        "feedback-001",
		Kind:        KindFeedback,
		Description: "low severity test",
		Severity:    SeverityLow,
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}

	// then: file exists in archive/ and outbox/ (NOT pending/)
	archivePath := filepath.Join(root, "archive", "feedback-001.md")
	outboxPath := filepath.Join(root, "outbox", "feedback-001.md")
	pendingPath := filepath.Join(root, "pending", "feedback-001.md")
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("expected file in archive: %v", err)
	}
	if _, err := os.Stat(outboxPath); err != nil {
		t.Errorf("expected file in outbox: %v", err)
	}
	if _, err := os.Stat(pendingPath); err == nil {
		t.Error("expected file NOT in pending for LOW severity")
	}
}

func TestSaveDMail_Format(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	dmail := DMail{
		Name:        "feedback-001",
		Kind:        KindFeedback,
		Description: "format test",
		Body:        "# Some body\n",
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}

	// then: raw content starts with ---
	data, err := readArchiveFile(root, "feedback-001.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "---\n") {
		t.Errorf("expected file to start with '---', got: %s", string(data[:20]))
	}
}

func TestLoadDMail_Exists(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	dmail := DMail{
		Name:        "feedback-001",
		Kind:        KindFeedback,
		Description: "round trip",
		Severity:    SeverityHigh,
		Body:        "# Test body\n",
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}

	// when
	loaded, err := store.LoadDMail("feedback-001")

	// then
	if err != nil {
		t.Fatalf("LoadDMail failed: %v", err)
	}
	if loaded.Name != "feedback-001" {
		t.Errorf("expected name feedback-001, got %s", loaded.Name)
	}
	if loaded.Kind != KindFeedback {
		t.Errorf("expected kind feedback, got %s", loaded.Kind)
	}
	if loaded.Severity != SeverityHigh {
		t.Errorf("expected severity high, got %s", loaded.Severity)
	}
}

func TestLoadDMail_NotFound(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	_, err := store.LoadDMail("feedback-999")
	if err == nil {
		t.Error("expected error for non-existent D-Mail")
	}
}

func TestLoadAllDMails_Multiple(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	for _, d := range []DMail{
		{Name: "feedback-002", Kind: KindFeedback, Description: "second"},
		{Name: "feedback-001", Kind: KindFeedback, Description: "first"},
		{Name: "feedback-003", Kind: KindFeedback, Description: "third"},
	} {
		if err := store.SaveDMail(d); err != nil {
			t.Fatal(err)
		}
	}

	dmails, err := store.LoadAllDMails()
	if err != nil {
		t.Fatalf("LoadAllDMails failed: %v", err)
	}
	if len(dmails) != 3 {
		t.Fatalf("expected 3, got %d", len(dmails))
	}
	if dmails[0].Name != "feedback-001" {
		t.Errorf("expected first feedback-001, got %s", dmails[0].Name)
	}
	if dmails[2].Name != "feedback-003" {
		t.Errorf("expected last feedback-003, got %s", dmails[2].Name)
	}
}

func TestLoadAllDMails_Empty(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	dmails, err := store.LoadAllDMails()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(dmails) != 0 {
		t.Errorf("expected 0, got %d", len(dmails))
	}
}

func TestRouteDMail_SeverityMapping(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		expected DMailStatus
	}{
		{"low auto-sent", SeverityLow, DMailSent},
		{"medium auto-sent", SeverityMedium, DMailSent},
		{"high pending", SeverityHigh, DMailPending},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := RouteDMail(tt.severity)
			if status != tt.expected {
				t.Errorf("expected status %s, got %s", tt.expected, status)
			}
		})
	}
}

func TestLoadResolution_NotFound_ReturnsSentinelError(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	_, err := store.LoadResolution("feedback-999")
	if err == nil {
		t.Fatal("expected error for non-existent resolution")
	}
	if !errors.Is(err, ErrNoResolution) {
		t.Errorf("expected ErrNoResolution, got: %v", err)
	}
}

func TestLoadResolutions_CorruptedJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}

	// Write corrupted JSON to resolutions.json
	runDir := filepath.Join(root, ".run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "resolutions.json"), []byte("{invalid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewStateStore(root)
	_, err := store.LoadResolutions()
	if err == nil {
		t.Fatal("expected error for corrupted JSON")
	}
	// Must NOT be ErrNoResolution — it's a real parse error
	if errors.Is(err, ErrNoResolution) {
		t.Error("corrupted JSON should not return ErrNoResolution")
	}
}

func TestSaveResolution_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	res := Resolution{
		Name:   "feedback-001",
		Status: "approved",
		Action: "approve",
	}
	if err := store.SaveResolution(res); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadResolution("feedback-001")
	if err != nil {
		t.Fatalf("LoadResolution failed: %v", err)
	}
	if loaded.Status != "approved" {
		t.Errorf("expected status approved, got %s", loaded.Status)
	}
	if loaded.Action != "approve" {
		t.Errorf("expected action approve, got %s", loaded.Action)
	}
}

func TestSaveConsumed_RoundTrip(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	now := time.Now().UTC().Truncate(time.Second)
	records := []ConsumedRecord{
		{Name: "report-001", Kind: KindReport, ConsumedAt: now, Source: "report-001.md"},
		{Name: "report-002", Kind: KindReport, ConsumedAt: now, Source: "report-002.md"},
	}

	// when
	if err := store.SaveConsumed(records); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadConsumed()
	if err != nil {
		t.Fatal(err)
	}

	// then
	if len(loaded) != 2 {
		t.Fatalf("expected 2 records, got %d", len(loaded))
	}
	if loaded[0].Name != "report-001" {
		t.Errorf("expected report-001, got %s", loaded[0].Name)
	}
	if loaded[1].Kind != KindReport {
		t.Errorf("expected report kind, got %s", loaded[1].Kind)
	}
}

func TestLoadConsumed_Empty(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// when
	loaded, err := store.LoadConsumed()

	// then
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected empty slice, got %d", len(loaded))
	}
}

func TestSaveConsumed_Appends(t *testing.T) {
	// given: save first batch
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	now := time.Now().UTC().Truncate(time.Second)

	first := []ConsumedRecord{{Name: "report-001", Kind: KindReport, ConsumedAt: now, Source: "report-001.md"}}
	if err := store.SaveConsumed(first); err != nil {
		t.Fatal(err)
	}

	// when: save second batch
	second := []ConsumedRecord{{Name: "report-002", Kind: KindReport, ConsumedAt: now, Source: "report-002.md"}}
	if err := store.SaveConsumed(second); err != nil {
		t.Fatal(err)
	}

	// then
	loaded, err := store.LoadConsumed()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 records after two saves, got %d", len(loaded))
	}
}

func TestScanInbox_Empty(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// when
	dmails, err := store.ScanInbox()

	// then
	if err != nil {
		t.Fatal(err)
	}
	if len(dmails) != 0 {
		t.Fatalf("expected empty, got %d", len(dmails))
	}
}

func TestScanInbox_SingleReport(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// Drop a report into inbox/
	content := []byte("---\nname: report-001\nkind: report\ndescription: test report\n---\n\nReport body.\n")
	inboxPath := filepath.Join(root, "inbox", "report-001.md")
	if err := os.WriteFile(inboxPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	dmails, err := store.ScanInbox()

	// then: parsed correctly
	if err != nil {
		t.Fatal(err)
	}
	if len(dmails) != 1 {
		t.Fatalf("expected 1, got %d", len(dmails))
	}
	if dmails[0].Name != "report-001" {
		t.Errorf("expected report-001, got %s", dmails[0].Name)
	}
	if dmails[0].Kind != KindReport {
		t.Errorf("expected report kind, got %s", dmails[0].Kind)
	}

	// then: copied to archive/
	archivePath := filepath.Join(root, "archive", "report-001.md")
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("expected file in archive: %v", err)
	}

	// then: removed from inbox/
	if _, err := os.Stat(inboxPath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected file removed from inbox")
	}
}

func TestScanInbox_MultipleReports(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	for _, name := range []string{"report-001", "report-002", "report-003"} {
		content := fmt.Sprintf("---\nname: %s\nkind: report\ndescription: test\n---\n\nbody\n", name)
		os.WriteFile(filepath.Join(root, "inbox", name+".md"), []byte(content), 0o644)
	}

	// when
	dmails, err := store.ScanInbox()

	// then
	if err != nil {
		t.Fatal(err)
	}
	if len(dmails) != 3 {
		t.Fatalf("expected 3, got %d", len(dmails))
	}
	// Sorted by name
	if dmails[0].Name != "report-001" || dmails[2].Name != "report-003" {
		t.Errorf("expected sorted order, got %s, %s, %s", dmails[0].Name, dmails[1].Name, dmails[2].Name)
	}
	// inbox/ should be empty of .md files
	entries, _ := os.ReadDir(filepath.Join(root, "inbox"))
	mdCount := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			mdCount++
		}
	}
	if mdCount != 0 {
		t.Errorf("expected inbox empty, got %d .md files", mdCount)
	}
}

func TestScanInbox_InvalidFile(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	os.WriteFile(filepath.Join(root, "inbox", "bad.md"), []byte("not valid frontmatter"), 0o644)

	// when
	_, err := store.ScanInbox()

	// then
	if err == nil {
		t.Fatal("expected error for invalid file")
	}
	if !strings.Contains(err.Error(), "parse inbox file") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestScanInbox_AlreadyInArchive(t *testing.T) {
	// given: file already exists in archive
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	content := []byte("---\nname: report-001\nkind: report\ndescription: original\n---\n\nOriginal body.\n")
	os.WriteFile(filepath.Join(root, "archive", "report-001.md"), content, 0o644)

	// Drop a different version into inbox
	inboxContent := []byte("---\nname: report-001\nkind: report\ndescription: duplicate\n---\n\nDuplicate body.\n")
	os.WriteFile(filepath.Join(root, "inbox", "report-001.md"), inboxContent, 0o644)

	// when
	dmails, err := store.ScanInbox()

	// then: no error
	if err != nil {
		t.Fatal(err)
	}
	if len(dmails) != 1 {
		t.Fatalf("expected 1, got %d", len(dmails))
	}

	// then: archive file NOT overwritten
	archiveData, _ := os.ReadFile(filepath.Join(root, "archive", "report-001.md"))
	parsed, _ := ParseDMail(archiveData)
	if parsed.Description != "original" {
		t.Errorf("expected archive to keep 'original', got %q", parsed.Description)
	}

	// then: inbox file still removed
	if _, err := os.Stat(filepath.Join(root, "inbox", "report-001.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected inbox file removed")
	}
}

func TestScanInbox_SkipsNonMD(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	os.WriteFile(filepath.Join(root, "inbox", "readme.txt"), []byte("ignore me"), 0o644)

	// when
	dmails, err := store.ScanInbox()

	// then
	if err != nil {
		t.Fatal(err)
	}
	if len(dmails) != 0 {
		t.Fatalf("expected 0, got %d", len(dmails))
	}
}

// Helper functions for tests

func statDir(path string) (interface{ IsDir() bool }, error) {
	return os.Stat(path)
}

func readGitignore(root string) ([]byte, error) {
	return os.ReadFile(filepath.Join(root, ".gitignore"))
}

func readArchiveFile(root, filename string) ([]byte, error) {
	return os.ReadFile(filepath.Join(root, "archive", filename))
}
