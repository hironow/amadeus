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
