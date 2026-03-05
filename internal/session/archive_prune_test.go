package session

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/usecase/port"
)

func TestFindPruneCandidates_DirNotExist(t *testing.T) {
	// given
	dir := filepath.Join(t.TempDir(), "nonexistent")

	// when
	candidates, err := FindPruneCandidates(dir, 30*24*time.Hour)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if candidates != nil {
		t.Errorf("expected nil for missing directory, got %v", candidates)
	}
}

func TestFindPruneCandidates_EmptyDir(t *testing.T) {
	// given
	dir := t.TempDir()

	// when
	candidates, err := FindPruneCandidates(dir, 30*24*time.Hour)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if candidates == nil {
		t.Error("expected non-nil empty slice for existing directory, got nil")
	}
	if len(candidates) != 0 {
		t.Errorf("expected empty, got %v", candidates)
	}
}

func TestFindPruneCandidates_FiltersOldFiles(t *testing.T) {
	// given
	dir := t.TempDir()
	oldFile := filepath.Join(dir, "feedback-001.md")
	if err := os.WriteFile(oldFile, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Set mtime to 31 days ago
	oldTime := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	newFile := filepath.Join(dir, "feedback-002.md")
	if err := os.WriteFile(newFile, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when: prune files older than 30 days
	candidates, err := FindPruneCandidates(dir, 30*24*time.Hour)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if filepath.Base(candidates[0].Path) != "feedback-001.md" {
		t.Errorf("expected feedback-001.md, got %s", candidates[0].Path)
	}
}

func TestFindPruneCandidates_IgnoresNonMdFiles(t *testing.T) {
	// given
	dir := t.TempDir()
	txtFile := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(txtFile, []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(txtFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// when
	candidates, err := FindPruneCandidates(dir, 30*24*time.Hour)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if candidates == nil {
		t.Error("expected non-nil empty slice for existing directory, got nil")
	}
	if len(candidates) != 0 {
		t.Errorf("expected empty (non-md ignored), got %d", len(candidates))
	}
}

func TestFindPruneCandidates_RecentFilesOnly(t *testing.T) {
	// given: directory exists with only recent .md files
	dir := t.TempDir()
	recentFile := filepath.Join(dir, "feedback-001.md")
	if err := os.WriteFile(recentFile, []byte("recent"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	candidates, err := FindPruneCandidates(dir, 30*24*time.Hour)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if candidates == nil {
		t.Error("expected non-nil empty slice for existing directory with recent files, got nil")
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
}

func TestFindPruneCandidates_IgnoresDirectories(t *testing.T) {
	// given
	dir := t.TempDir()
	subDir := filepath.Join(dir, "subdir.md")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// when
	candidates, err := FindPruneCandidates(dir, 30*24*time.Hour)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected empty (dirs ignored), got %d", len(candidates))
	}
}

func TestPruneFiles_DeletesFiles(t *testing.T) {
	// given
	dir := t.TempDir()
	f1 := filepath.Join(dir, "feedback-001.md")
	f2 := filepath.Join(dir, "feedback-002.md")
	if err := os.WriteFile(f1, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	candidates := []port.PruneCandidate{
		{Path: f1, ModTime: time.Now()},
		{Path: f2, ModTime: time.Now()},
	}

	// when
	count, err := PruneFiles(candidates)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 deleted, got %d", count)
	}
	if _, err := os.Stat(f1); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected %s to be deleted", f1)
	}
	if _, err := os.Stat(f2); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected %s to be deleted", f2)
	}
}

func TestPruneFiles_EmptyList(t *testing.T) {
	// when
	count, err := PruneFiles(nil)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}
