package eventsource

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindExpiredEventFiles_DirNotExist(t *testing.T) {
	// given
	dir := filepath.Join(t.TempDir(), "nonexistent")

	// when
	files, err := FindExpiredEventFiles(dir, 30*24*time.Hour)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if files != nil {
		t.Errorf("expected nil for missing directory, got %v", files)
	}
}

func TestFindExpiredEventFiles_EmptyDir(t *testing.T) {
	// given
	dir := t.TempDir()

	// when
	files, err := FindExpiredEventFiles(dir, 30*24*time.Hour)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if files != nil {
		t.Errorf("expected nil for empty directory, got %v", files)
	}
}

func TestFindExpiredEventFiles_FiltersOldJsonlFiles(t *testing.T) {
	// given
	dir := t.TempDir()
	oldFile := filepath.Join(dir, "2026-01-01.jsonl")
	if err := os.WriteFile(oldFile, []byte(`{"id":"1"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	newFile := filepath.Join(dir, "2026-02-25.jsonl")
	if err := os.WriteFile(newFile, []byte(`{"id":"2"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	files, err := FindExpiredEventFiles(dir, 30*24*time.Hour)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 expired file, got %d", len(files))
	}
	if filepath.Base(files[0].Path) != "2026-01-01.jsonl" {
		t.Errorf("expected 2026-01-01.jsonl, got %s", files[0].Path)
	}
}

func TestFindExpiredEventFiles_IgnoresNonJsonlFiles(t *testing.T) {
	// given
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "feedback-001.md")
	if err := os.WriteFile(mdFile, []byte("markdown"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(mdFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// when
	files, err := FindExpiredEventFiles(dir, 30*24*time.Hour)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files (non-jsonl ignored), got %d", len(files))
	}
}

func TestPruneEventFiles_DeletesFiles(t *testing.T) {
	// given
	dir := t.TempDir()
	f1 := filepath.Join(dir, "2026-01-01.jsonl")
	if err := os.WriteFile(f1, []byte(`{"id":"1"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files := []ExpiredFile{
		{Path: f1, ModTime: time.Now()},
	}

	// when
	count, err := PruneEventFiles(files)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 deleted, got %d", count)
	}
	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Errorf("expected %s to be deleted", f1)
	}
}
