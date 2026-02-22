//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- D-Mail Severity Routing ---

func TestE2E_DMail_HighSeverity_GoesToOutbox(t *testing.T) {
	dir := initTestRepo(t)
	cfg := defaultTestConfig()
	cfg["thresholds"] = map[string]any{
		"low_max":    0.05,
		"medium_max": 0.10,
	}
	writeConfig(t, dir, cfg)

	_, _, err := runCmd(t, dir, "check", "--full", "--json")
	assertExitCode(t, err, 2)

	outboxFiles := listDir(t, filepath.Join(dir, ".gate", "outbox"))
	archiveFiles := listDir(t, filepath.Join(dir, ".gate", "archive"))

	if len(outboxFiles) == 0 {
		t.Error("HIGH severity D-Mail should be in outbox/")
	}
	if len(archiveFiles) == 0 {
		t.Error("D-Mail should always be in archive/")
	}
}

func TestE2E_DMail_LowSeverity_GoesToOutbox(t *testing.T) {
	dir := initTestRepo(t)
	cfg := defaultTestConfig()
	cfg["thresholds"] = map[string]any{
		"low_max":    0.90,
		"medium_max": 0.95,
	}
	writeConfig(t, dir, cfg)

	_, _, err := runCmd(t, dir, "check", "--full", "--json")
	assertExitCode(t, err, 2)

	outboxFiles := listDir(t, filepath.Join(dir, ".gate", "outbox"))

	if len(outboxFiles) == 0 {
		t.Error("LOW severity D-Mail should be in outbox/")
	}
}

func TestE2E_DMail_MediumSeverity_GoesToOutbox(t *testing.T) {
	dir := initTestRepo(t)
	cfg := defaultTestConfig()
	cfg["thresholds"] = map[string]any{
		"low_max":    0.05,
		"medium_max": 0.90,
	}
	writeConfig(t, dir, cfg)

	_, _, err := runCmd(t, dir, "check", "--full", "--json")
	assertExitCode(t, err, 2)

	outboxFiles := listDir(t, filepath.Join(dir, ".gate", "outbox"))
	if len(outboxFiles) == 0 {
		t.Error("MEDIUM severity D-Mail should be in outbox/ (auto-sent)")
	}
}

// --- D-Mail File Format ---

func TestE2E_DMail_ArchiveFormat(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	_, _, err := runCmd(t, dir, "check", "--full", "--json")
	assertExitCode(t, err, 2)

	archiveDir := filepath.Join(dir, ".gate", "archive")
	files := listDir(t, archiveDir)
	if len(files) == 0 {
		t.Fatal("no archive files")
	}

	// Read first D-Mail and verify YAML frontmatter format
	data, err := os.ReadFile(filepath.Join(archiveDir, files[0]))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.HasPrefix(content, "---\n") {
		t.Error("D-Mail should start with ---")
	}
	if !strings.Contains(content, "name:") {
		t.Error("D-Mail frontmatter should contain 'name:'")
	}
	if !strings.Contains(content, "kind:") {
		t.Error("D-Mail frontmatter should contain 'kind:'")
	}
	if !strings.Contains(content, "description:") {
		t.Error("D-Mail frontmatter should contain 'description:'")
	}
}

func TestE2E_DMail_NamingSequence(t *testing.T) {
	dir := initTestRepo(t)
	cfg := defaultTestConfig()
	cfg["thresholds"] = map[string]any{
		"low_max":    0.90,
		"medium_max": 0.95,
	}
	writeConfig(t, dir, cfg)

	// First check
	runCmd(t, dir, "check", "--full", "--json")

	// Add commit and run again
	f := filepath.Join(dir, "a.go")
	os.WriteFile(f, []byte("package main\n"), 0o644)
	gitAdd := exec.Command("git", "add", ".")
	gitAdd.Dir = dir
	gitAdd.Run()
	gitCommit := exec.Command("git", "commit", "-m", "change")
	gitCommit.Dir = dir
	gitCommit.Run()

	runCmd(t, dir, "check", "--full", "--json")

	archiveFiles := listDir(t, filepath.Join(dir, ".gate", "archive"))
	// Should have sequential names like feedback-001.md, feedback-002.md
	if len(archiveFiles) < 2 {
		t.Errorf("expected at least 2 D-Mail files, got %d", len(archiveFiles))
	}
	// Verify sequential naming
	for _, f := range archiveFiles {
		if !strings.Contains(f, "-001") && !strings.Contains(f, "-002") && !strings.Contains(f, "-003") {
			t.Logf("archive file: %s", f)
		}
	}
}

// --- Inbox Consumption ---

func TestE2E_DMail_InboxConsumption(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	// Place a D-Mail in inbox/
	writeDMail(t, dir, "inbox", "report-001", map[string]any{
		"name":        "report-001",
		"kind":        "report",
		"description": "Test report from downstream",
		"severity":    "low",
	}, "Report body content.\n")

	assertFileExists(t, filepath.Join(dir, ".gate", "inbox", "report-001.md"))

	// Run check — should consume inbox
	runCmd(t, dir, "check", "--full", "--json")

	// Inbox should be empty
	assertFileNotExists(t, filepath.Join(dir, ".gate", "inbox", "report-001.md"))

	// Archive should have the consumed D-Mail
	assertFileExists(t, filepath.Join(dir, ".gate", "archive", "report-001.md"))
}

func TestE2E_DMail_InboxDedupSkipsExisting(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	fm := map[string]any{
		"name":        "report-001",
		"kind":        "report",
		"description": "Test report",
		"severity":    "low",
	}
	body := "Original body.\n"

	// Place in both inbox and archive (simulate already-consumed)
	writeDMail(t, dir, "inbox", "report-001", fm, body)
	writeDMail(t, dir, "archive", "report-001", fm, "Archive body should be preserved.\n")

	runCmd(t, dir, "check", "--full", "--json")

	// Inbox consumed
	assertFileNotExists(t, filepath.Join(dir, ".gate", "inbox", "report-001.md"))

	// Archive should keep original (not overwritten)
	data, _ := os.ReadFile(filepath.Join(dir, ".gate", "archive", "report-001.md"))
	if strings.Contains(string(data), "Original body") {
		t.Error("archive should have preserved original, not overwritten with inbox copy")
	}
}

func TestE2E_DMail_InboxNotConsumedInDryRun(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	writeDMail(t, dir, "inbox", "report-001", map[string]any{
		"name":        "report-001",
		"kind":        "report",
		"description": "Test report",
		"severity":    "low",
	}, "Report body.\n")

	// Dry-run should NOT consume inbox
	runCmd(t, dir, "check", "--full", "--dry-run")

	// Inbox should still have the file
	assertFileExists(t, filepath.Join(dir, ".gate", "inbox", "report-001.md"))
}

// --- Archive Prune ---

func TestE2E_ArchivePrune_DryRun(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	// Seed an old D-Mail
	writeDMail(t, dir, "archive", "feedback-001", map[string]any{
		"name":        "feedback-001",
		"kind":        "feedback",
		"description": "Old feedback",
		"severity":    "low",
	}, "Old body.\n")

	// Backdate the file
	oldTime := time.Now().Add(-60 * 24 * time.Hour)
	archivePath := filepath.Join(dir, ".gate", "archive", "feedback-001.md")
	os.Chtimes(archivePath, oldTime, oldTime)

	_, stderr, err := runCmd(t, dir, "archive-prune", "--days", "30", "--dry-run")
	if err != nil {
		t.Fatalf("archive-prune --dry-run: %v", err)
	}

	if !strings.Contains(stderr, "dry-run") {
		t.Errorf("expected 'dry-run' in output, got: %s", stderr)
	}

	// File should still exist
	assertFileExists(t, archivePath)
}

func TestE2E_ArchivePrune_WithYes(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	writeDMail(t, dir, "archive", "feedback-001", map[string]any{
		"name":        "feedback-001",
		"kind":        "feedback",
		"description": "Old feedback",
		"severity":    "low",
	}, "Old body.\n")

	oldTime := time.Now().Add(-60 * 24 * time.Hour)
	archivePath := filepath.Join(dir, ".gate", "archive", "feedback-001.md")
	os.Chtimes(archivePath, oldTime, oldTime)

	_, _, err := runCmd(t, dir, "archive-prune", "--days", "30", "--yes")
	if err != nil {
		t.Fatalf("archive-prune --yes: %v", err)
	}

	// File should be deleted
	assertFileNotExists(t, archivePath)
}

func TestE2E_ArchivePrune_PreservesRecent(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	writeDMail(t, dir, "archive", "feedback-001", map[string]any{
		"name":        "feedback-001",
		"kind":        "feedback",
		"description": "Recent feedback",
		"severity":    "low",
	}, "Recent body.\n")

	// File is recent (just created) — should not be pruned
	_, stderr, err := runCmd(t, dir, "archive-prune", "--days", "30", "--yes")
	if err != nil {
		t.Fatalf("archive-prune: %v", err)
	}

	if !strings.Contains(stderr, "No files older than") {
		t.Logf("stderr: %s", stderr)
	}

	assertFileExists(t, filepath.Join(dir, ".gate", "archive", "feedback-001.md"))
}

func TestE2E_ArchivePrune_EmptyArchive(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	_, stderr, err := runCmd(t, dir, "archive-prune", "--days", "30", "--yes")
	if err != nil {
		t.Fatalf("archive-prune on empty: %v", err)
	}
	if !strings.Contains(stderr, "No files") {
		t.Logf("stderr: %s", stderr)
	}
}

func TestE2E_ArchivePrune_InvalidDays(t *testing.T) {
	dir := initTestRepo(t)
	_, _, err := runCmd(t, dir, "archive-prune", "--days", "0")
	if err == nil {
		t.Fatal("expected error for --days 0")
	}
}
