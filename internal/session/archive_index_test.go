package session_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestExtractSummary_HeadingLine(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	os.WriteFile(f, []byte("some preamble\n# My Heading\nbody text\n"), 0644)

	got := session.ExtractSummary(f)
	if got != "My Heading" {
		t.Errorf("got %q, want %q", got, "My Heading")
	}
}

func TestExtractSummary_NoHeading(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	os.WriteFile(f, []byte("\nfirst non-empty line\nsecond line\n"), 0644)

	got := session.ExtractSummary(f)
	if got != "first non-empty line" {
		t.Errorf("got %q, want %q", got, "first non-empty line")
	}
}

func TestExtractSummary_Truncate100(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	long := "# " + strings.Repeat("x", 200)
	os.WriteFile(f, []byte(long), 0644)

	got := session.ExtractSummary(f)
	if len(got) > 100 {
		t.Errorf("summary length %d exceeds 100", len(got))
	}
}

func TestExtractSummary_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	os.WriteFile(f, []byte(""), 0644)

	got := session.ExtractSummary(f)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractSummary_FrontmatterSkipped(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	content := "---\nname: test\nkind: report\n---\n# Report Title\nbody\n"
	os.WriteFile(f, []byte(content), 0644)

	got := session.ExtractSummary(f)
	if got != "Report Title" {
		t.Errorf("got %q, want %q", got, "Report Title")
	}
}

func TestExtractMeta_DmailWithFrontmatter(t *testing.T) {
	stateDir := t.TempDir()
	archiveDir := filepath.Join(stateDir, "archive")
	os.MkdirAll(archiveDir, 0755)
	f := filepath.Join(archiveDir, "report-auth-fix-cluster-w1.md")
	content := "---\nname: report-auth-fix-cluster-w1\nkind: implementation-feedback\ndescription: Auth fix feedback\ndmail-schema-version: \"1\"\nissues:\n  - ENG-456\nmetadata:\n  idempotency_key: abc123\n---\n# Auth Fix Report\nRecommended actions...\n"
	os.WriteFile(f, []byte(content), 0644)

	entry := session.ExtractMeta(f, stateDir, "amadeus")
	if entry.Operation != "dmail" {
		t.Errorf("op: got %q, want %q", entry.Operation, "dmail")
	}
	if entry.Issue != "ENG-456" {
		t.Errorf("issue: got %q, want %q", entry.Issue, "ENG-456")
	}
	if entry.Tool != "amadeus" {
		t.Errorf("tool: got %q, want %q", entry.Tool, "amadeus")
	}
	if entry.Summary != "Auth Fix Report" {
		t.Errorf("summary: got %q, want %q", entry.Summary, "Auth Fix Report")
	}
	if entry.Path != "archive/report-auth-fix-cluster-w1.md" {
		t.Errorf("path: got %q, want %q", entry.Path, "archive/report-auth-fix-cluster-w1.md")
	}
}

func TestExtractMeta_JournalFile(t *testing.T) {
	stateDir := t.TempDir()
	journalDir := filepath.Join(stateDir, "journal")
	os.MkdirAll(journalDir, 0755)
	f := filepath.Join(journalDir, "001.md")
	content := "# Expedition #1 — Journal\n\n- **Date**: 2026-03-09 22:03:22\n- **Issue**: ENG-789 — Fix login bug\n- **Status**: failed\n- **Reason**: exit status 1\n"
	os.WriteFile(f, []byte(content), 0644)

	entry := session.ExtractMeta(f, stateDir, "paintress")
	if entry.Operation != "expedition" {
		t.Errorf("op: got %q, want %q", entry.Operation, "expedition")
	}
	if entry.Issue != "ENG-789" {
		t.Errorf("issue: got %q, want %q", entry.Issue, "ENG-789")
	}
	if entry.Status != "failed" {
		t.Errorf("status: got %q, want %q", entry.Status, "failed")
	}
	if entry.Timestamp != "2026-03-09T22:03:22Z" {
		t.Errorf("ts: got %q, want %q", entry.Timestamp, "2026-03-09T22:03:22Z")
	}
	if entry.Path != "journal/001.md" {
		t.Errorf("path: got %q, want %q", entry.Path, "journal/001.md")
	}
}

func TestExtractMeta_InsightFile(t *testing.T) {
	stateDir := t.TempDir()
	insightsDir := filepath.Join(stateDir, "insights")
	os.MkdirAll(insightsDir, 0755)
	f := filepath.Join(insightsDir, "gommage.md")
	content := "---\ninsight-schema-version: \"1\"\nkind: gommage\ntool: amadeus\nupdated_at: \"2026-03-13T13:55:50+09:00\"\nentries: 59\n---\n## Insight: Cache invalidation pattern\n...\n"
	os.WriteFile(f, []byte(content), 0644)

	entry := session.ExtractMeta(f, stateDir, "amadeus")
	if entry.Operation != "divergence" {
		t.Errorf("op: got %q, want %q", entry.Operation, "divergence")
	}
	if entry.Timestamp != "2026-03-13T13:55:50+09:00" {
		t.Errorf("ts: got %q, want %q", entry.Timestamp, "2026-03-13T13:55:50+09:00")
	}
	if entry.Path != "insights/gommage.md" {
		t.Errorf("path: got %q, want %q", entry.Path, "insights/gommage.md")
	}
}

func TestExtractMeta_NoIssueID(t *testing.T) {
	stateDir := t.TempDir()
	archiveDir := filepath.Join(stateDir, "archive")
	os.MkdirAll(archiveDir, 0755)
	f := filepath.Join(archiveDir, "generic-report.md")
	os.WriteFile(f, []byte("# Generic Report\nNo issue mentioned.\n"), 0644)

	entry := session.ExtractMeta(f, stateDir, "sightjack")
	if entry.Issue != "" {
		t.Errorf("issue: got %q, want empty", entry.Issue)
	}
}

func TestIndexWriter_Append_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.jsonl")

	w := &session.IndexWriter{}
	entries := []domain.IndexEntry{
		{Timestamp: "2026-03-10T14:30:00Z", Operation: "divergence", Issue: "ENG-1", Status: "success", Tool: "amadeus", Path: "archive/report.md", Summary: "test"},
	}

	if err := w.Append(indexPath, entries); err != nil {
		t.Fatalf("append: %v", err)
	}

	data, _ := os.ReadFile(indexPath)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var got domain.IndexEntry
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Issue != "ENG-1" {
		t.Errorf("issue: got %q, want %q", got.Issue, "ENG-1")
	}
}

func TestIndexWriter_Append_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.jsonl")

	w := &session.IndexWriter{}
	e1 := []domain.IndexEntry{{Timestamp: "t1", Operation: "o1", Tool: "amadeus", Path: "p1", Summary: "s1"}}
	e2 := []domain.IndexEntry{{Timestamp: "t2", Operation: "o2", Tool: "amadeus", Path: "p2", Summary: "s2"}}

	w.Append(indexPath, e1)
	w.Append(indexPath, e2)

	data, _ := os.ReadFile(indexPath)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestIndexWriter_Append_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.jsonl")

	w := &session.IndexWriter{}
	entries := []domain.IndexEntry{
		{Timestamp: "t1", Operation: "o1", Tool: "amadeus", Path: "p1", Summary: "summary with \"quotes\""},
		{Timestamp: "t2", Operation: "o2", Tool: "amadeus", Path: "p2", Summary: "s2"},
	}
	w.Append(indexPath, entries)

	data, _ := os.ReadFile(indexPath)
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var e domain.IndexEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Errorf("line %d invalid JSON: %v", i, err)
		}
	}
}

func TestIndexWriter_Append_EmptyEntries(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.jsonl")

	w := &session.IndexWriter{}
	if err := w.Append(indexPath, nil); err != nil {
		t.Fatalf("append nil: %v", err)
	}

	_, statErr := os.Stat(indexPath)
	if !os.IsNotExist(statErr) {
		t.Errorf("expected no file for empty entries")
	}
}

func TestIndexWriter_Append_ConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.jsonl")

	w := &session.IndexWriter{}
	const goroutines = 10
	const entriesPerGoroutine = 5

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			var entries []domain.IndexEntry
			for j := 0; j < entriesPerGoroutine; j++ {
				entries = append(entries, domain.IndexEntry{
					Timestamp: fmt.Sprintf("t-%d-%d", id, j),
					Operation: "divergence",
					Tool:      "amadeus",
					Path:      fmt.Sprintf("p-%d-%d", id, j),
					Summary:   fmt.Sprintf("s-%d-%d", id, j),
				})
			}
			if err := w.Append(indexPath, entries); err != nil {
				t.Errorf("goroutine %d: %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	expected := goroutines * entriesPerGoroutine
	if len(lines) != expected {
		t.Fatalf("expected %d lines, got %d", expected, len(lines))
	}

	for i, line := range lines {
		var e domain.IndexEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Errorf("line %d invalid JSON: %v\nline: %s", i, err, line)
		}
	}
}

func TestIndexWriter_Rebuild_ScansAllMd(t *testing.T) {
	stateDir := t.TempDir()
	archiveDir := filepath.Join(stateDir, "archive")
	journalDir := filepath.Join(stateDir, "journal")
	os.MkdirAll(archiveDir, 0755)
	os.MkdirAll(journalDir, 0755)

	os.WriteFile(filepath.Join(archiveDir, "report-a.md"), []byte("# Report A\n"), 0644)
	os.WriteFile(filepath.Join(journalDir, "001.md"), []byte("# Journal 1\n"), 0644)
	os.WriteFile(filepath.Join(archiveDir, "data.jsonl"), []byte("not indexed\n"), 0644)

	indexPath := filepath.Join(stateDir, "archive", "index.jsonl")
	w := &session.IndexWriter{}
	n, err := w.Rebuild(indexPath, stateDir, "amadeus")
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 entries, got %d", n)
	}
}

func TestIndexWriter_Rebuild_OverwritesExisting(t *testing.T) {
	stateDir := t.TempDir()
	archiveDir := filepath.Join(stateDir, "archive")
	os.MkdirAll(archiveDir, 0755)
	os.WriteFile(filepath.Join(archiveDir, "report.md"), []byte("# Only Report\n"), 0644)

	indexPath := filepath.Join(archiveDir, "index.jsonl")
	os.WriteFile(indexPath, []byte("{\"ts\":\"old\"}\n{\"ts\":\"old2\"}\n"), 0644)

	w := &session.IndexWriter{}
	n, _ := w.Rebuild(indexPath, stateDir, "amadeus")
	if n != 1 {
		t.Errorf("expected 1 entry, got %d", n)
	}

	data, _ := os.ReadFile(indexPath)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line after rebuild, got %d", len(lines))
	}
}

func TestIndexWriter_Rebuild_EmptyDir(t *testing.T) {
	stateDir := t.TempDir()
	archiveDir := filepath.Join(stateDir, "archive")
	os.MkdirAll(archiveDir, 0755)

	indexPath := filepath.Join(archiveDir, "index.jsonl")
	w := &session.IndexWriter{}
	n, err := w.Rebuild(indexPath, stateDir, "amadeus")
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 entries, got %d", n)
	}
}

func TestIndexWriter_Rebuild_MissingDirs(t *testing.T) {
	stateDir := t.TempDir()
	indexPath := filepath.Join(stateDir, "archive", "index.jsonl")

	w := &session.IndexWriter{}
	n, err := w.Rebuild(indexPath, stateDir, "phonewave")
	if err != nil {
		t.Fatalf("rebuild with missing dirs should succeed: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 entries, got %d", n)
	}
}

func TestIndexWriter_Rebuild_SkipsIndexFile(t *testing.T) {
	stateDir := t.TempDir()
	archiveDir := filepath.Join(stateDir, "archive")
	os.MkdirAll(archiveDir, 0755)
	os.WriteFile(filepath.Join(archiveDir, "report.md"), []byte("# Report\n"), 0644)

	indexPath := filepath.Join(archiveDir, "index.jsonl")
	w := &session.IndexWriter{}
	n, _ := w.Rebuild(indexPath, stateDir, "amadeus")
	if n != 1 {
		t.Errorf("expected 1 entry (not including index.jsonl), got %d", n)
	}
}
