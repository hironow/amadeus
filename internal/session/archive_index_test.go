package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
