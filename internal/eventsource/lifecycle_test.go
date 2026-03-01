package eventsource

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListExpiredEventFiles_DirNotExist(t *testing.T) {
	// given
	stateDir := filepath.Join(t.TempDir(), "nonexistent")

	// when
	files, err := ListExpiredEventFiles(stateDir, 30)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if files != nil {
		t.Errorf("expected nil for missing directory, got %v", files)
	}
}

func TestListExpiredEventFiles_EmptyDir(t *testing.T) {
	// given
	stateDir := t.TempDir()
	eventsDir := filepath.Join(stateDir, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// when
	files, err := ListExpiredEventFiles(stateDir, 30)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if files != nil {
		t.Errorf("expected nil for empty directory, got %v", files)
	}
}

func TestListExpiredEventFiles_FiltersOldJsonlFiles(t *testing.T) {
	// given
	stateDir := t.TempDir()
	eventsDir := filepath.Join(stateDir, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldFile := filepath.Join(eventsDir, "2026-01-01.jsonl")
	if err := os.WriteFile(oldFile, []byte(`{"id":"1"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	newFile := filepath.Join(eventsDir, "2026-02-25.jsonl")
	if err := os.WriteFile(newFile, []byte(`{"id":"2"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	files, err := ListExpiredEventFiles(stateDir, 30)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 expired file, got %d", len(files))
	}
	if files[0] != "2026-01-01.jsonl" {
		t.Errorf("expected 2026-01-01.jsonl, got %s", files[0])
	}
}

func TestListExpiredEventFiles_IgnoresNonJsonlFiles(t *testing.T) {
	// given
	stateDir := t.TempDir()
	eventsDir := filepath.Join(stateDir, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mdFile := filepath.Join(eventsDir, "feedback-001.md")
	if err := os.WriteFile(mdFile, []byte("markdown"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(mdFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// when
	files, err := ListExpiredEventFiles(stateDir, 30)

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
	stateDir := t.TempDir()
	eventsDir := filepath.Join(stateDir, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	f1 := filepath.Join(eventsDir, "2026-01-01.jsonl")
	if err := os.WriteFile(f1, []byte(`{"id":"1"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	deleted, err := PruneEventFiles(stateDir, []string{"2026-01-01.jsonl"})

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(deleted) != 1 {
		t.Errorf("expected 1 deleted, got %d", len(deleted))
	}
	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Errorf("expected %s to be deleted", f1)
	}
}
