package eventsource

// white-box-reason: eventsource internals: tests unexported file rotation and lifecycle logic

import (
	"errors"
	"io/fs"
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
	if _, err := os.Stat(f1); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected %s to be deleted", f1)
	}
}

func TestListOversizedEventFiles_ReturnsBigFiles(t *testing.T) {
	// given
	stateDir := t.TempDir()
	eventsDir := filepath.Join(stateDir, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// today's file — must be excluded even if oversized
	today := time.Now().Format("2006-01-02")
	todayFile := filepath.Join(eventsDir, today+".jsonl")
	bigContent := make([]byte, EventFileSizeThreshold+1)
	if err := os.WriteFile(todayFile, bigContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// yesterday big file — must be included
	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
	yesterdayFile := filepath.Join(eventsDir, yesterday+".jsonl")
	if err := os.WriteFile(yesterdayFile, bigContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// small file — must be excluded
	smallFile := filepath.Join(eventsDir, "2026-01-01.jsonl")
	if err := os.WriteFile(smallFile, []byte(`{"id":"1"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	files, err := ListOversizedEventFiles(stateDir)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 oversized file, got %d: %v", len(files), files)
	}
	if files[0] != yesterday+".jsonl" {
		t.Errorf("expected %s.jsonl, got %s", yesterday, files[0])
	}
}

func TestListOversizedEventFiles_DirNotExist(t *testing.T) {
	// given
	stateDir := filepath.Join(t.TempDir(), "nonexistent")

	// when
	files, err := ListOversizedEventFiles(stateDir)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if files != nil {
		t.Errorf("expected nil for missing directory, got %v", files)
	}
}

func TestTruncateEventFile_KeepsLastNLines(t *testing.T) {
	// given
	stateDir := t.TempDir()
	eventsDir := filepath.Join(stateDir, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	lines := []string{
		`{"id":"1"}`,
		`{"id":"2"}`,
		`{"id":"3"}`,
		`{"id":"4"}`,
		`{"id":"5"}`,
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	fname := "2026-01-15.jsonl"
	fpath := filepath.Join(eventsDir, fname)
	if err := os.WriteFile(fpath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// when — keep last 3 lines
	if err := TruncateEventFile(stateDir, fname, 3); err != nil {
		t.Fatalf("TruncateEventFile: %v", err)
	}

	// then
	data, readErr := os.ReadFile(fpath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	got := string(data)
	for _, want := range []string{`{"id":"3"}`, `{"id":"4"}`, `{"id":"5"}`} {
		if !containsLine(got, want) {
			t.Errorf("expected line %q in result, got:\n%s", want, got)
		}
	}
	for _, notWant := range []string{`{"id":"1"}`, `{"id":"2"}`} {
		if containsLine(got, notWant) {
			t.Errorf("expected line %q to be removed, got:\n%s", notWant, got)
		}
	}
}

func containsLine(text, line string) bool {
	for _, l := range splitLines(text) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if line != "" {
				out = append(out, line)
			}
			start = i + 1
		}
	}
	if start < len(s) && s[start:] != "" {
		out = append(out, s[start:])
	}
	return out
}
