//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Archive Prune ---

func TestE2E_ArchivePrune_DryRun(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	// Seed an old D-Mail
	writeDMail(t, dir, "archive", "feedback-001", map[string]any{
		"name":        "feedback-001",
		"kind":        "design-feedback",
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
		"kind":        "design-feedback",
		"description": "Old feedback",
		"severity":    "low",
	}, "Old body.\n")

	oldTime := time.Now().Add(-60 * 24 * time.Hour)
	archivePath := filepath.Join(dir, ".gate", "archive", "feedback-001.md")
	os.Chtimes(archivePath, oldTime, oldTime)

	_, _, err := runCmd(t, dir, "archive-prune", "--execute", "--days", "30", "--yes")
	if err != nil {
		t.Fatalf("archive-prune --execute --yes: %v", err)
	}

	// File should be deleted
	assertFileNotExists(t, archivePath)
}

func TestE2E_ArchivePrune_PreservesRecent(t *testing.T) {
	dir := initTestRepo(t)
	writeConfig(t, dir, defaultTestConfig())

	writeDMail(t, dir, "archive", "feedback-001", map[string]any{
		"name":        "feedback-001",
		"kind":        "design-feedback",
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
