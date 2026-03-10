package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestInsightWriter_WriteNew(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)

	entry := domain.InsightEntry{
		Title:       "test insight",
		What:        "observed X",
		Why:         "because Y",
		How:         "do Z",
		When:        "always",
		Who:         "test",
		Constraints: "none",
	}

	err := w.Append("test.md", "test-kind", "test-tool", entry)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Verify file was created
	data, err := os.ReadFile(filepath.Join(insightsDir, "test.md"))
	if err != nil {
		t.Fatalf("read insight file: %v", err)
	}

	file, err := domain.UnmarshalInsightFile(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(file.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.Entries))
	}
	if file.Entries[0].Title != "test insight" {
		t.Errorf("expected title 'test insight', got %q", file.Entries[0].Title)
	}
	if file.Kind != "test-kind" {
		t.Errorf("expected kind 'test-kind', got %q", file.Kind)
	}
}

func TestInsightWriter_AppendExisting(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)

	e1 := domain.InsightEntry{Title: "first", What: "a", Why: "b", How: "c", When: "d", Who: "e", Constraints: "f"}
	e2 := domain.InsightEntry{Title: "second", What: "g", Why: "h", How: "i", When: "j", Who: "k", Constraints: "l"}

	if err := w.Append("multi.md", "lumina", "paintress", e1); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := w.Append("multi.md", "lumina", "paintress", e2); err != nil {
		t.Fatalf("second append: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(insightsDir, "multi.md"))
	file, _ := domain.UnmarshalInsightFile(data)

	if len(file.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(file.Entries))
	}
	if file.Entries[0].Title != "first" {
		t.Errorf("first entry title: %q", file.Entries[0].Title)
	}
	if file.Entries[1].Title != "second" {
		t.Errorf("second entry title: %q", file.Entries[1].Title)
	}
}

func TestInsightWriter_AtomicNoCorruption(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)
	entry := domain.InsightEntry{Title: "atomic", What: "a", Why: "b", How: "c", When: "d", Who: "e", Constraints: "f"}

	_ = w.Append("atomic.md", "test", "test", entry)

	// No temp files should remain
	matches, _ := filepath.Glob(filepath.Join(insightsDir, ".*.tmp"))
	if len(matches) > 0 {
		t.Errorf("temp files should be cleaned up, found: %v", matches)
	}
}

func TestInsightWriter_IdempotentAppend(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)

	entry := domain.InsightEntry{Title: "dedup me", What: "a", Why: "b", How: "c", When: "d", Who: "e", Constraints: "f"}

	// Append twice with same title
	if err := w.Append("dedup.md", "test", "test", entry); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := w.Append("dedup.md", "test", "test", entry); err != nil {
		t.Fatalf("second append: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(insightsDir, "dedup.md"))
	file, _ := domain.UnmarshalInsightFile(data)

	if len(file.Entries) != 1 {
		t.Errorf("expected 1 entry (idempotent), got %d", len(file.Entries))
	}
}

func TestInsightWriter_PropagatesNonENOENT(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	// Create a corrupt file (invalid YAML frontmatter)
	os.WriteFile(filepath.Join(insightsDir, "corrupt.md"), []byte("not valid insight file"), 0o644)

	w := session.NewInsightWriter(insightsDir, runDir)
	entry := domain.InsightEntry{Title: "test", What: "a", Why: "b", How: "c", When: "d", Who: "e", Constraints: "f"}

	err := w.Append("corrupt.md", "test", "test", entry)
	if err == nil {
		t.Fatal("expected error for corrupt file, got nil")
	}
}

func TestInsightWriter_ReadEntries(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)
	entry := domain.InsightEntry{Title: "readable", What: "a", Why: "b", How: "c", When: "d", Who: "e", Constraints: "f"}
	_ = w.Append("read.md", "lumina", "paintress", entry)

	file, err := w.Read("read.md")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(file.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.Entries))
	}
}
