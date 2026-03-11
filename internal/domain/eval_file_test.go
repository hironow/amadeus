package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestValidateFilesRead_AllPresent(t *testing.T) {
	// given
	got := []string{"adrs", "dods", "diff", "previous_scores"}
	expected := []string{"adrs", "dods", "diff", "previous_scores"}

	// when
	err := domain.ValidateFilesRead(got, expected)

	// then
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateFilesRead_MissingOne(t *testing.T) {
	// given
	got := []string{"adrs", "dods", "diff"}
	expected := []string{"adrs", "dods", "diff", "previous_scores"}

	// when
	err := domain.ValidateFilesRead(got, expected)

	// then
	if err == nil {
		t.Fatal("expected error for missing kind, got nil")
	}
	if !errors.Is(err, domain.ErrIncompleteRead) {
		t.Fatalf("expected ErrIncompleteRead, got %v", err)
	}
}

func TestValidateFilesRead_EmptyGot(t *testing.T) {
	// given
	var got []string
	expected := []string{"adrs"}

	// when
	err := domain.ValidateFilesRead(got, expected)

	// then
	if !errors.Is(err, domain.ErrIncompleteRead) {
		t.Fatalf("expected ErrIncompleteRead, got %v", err)
	}
}

func TestValidateFilesRead_EmptyExpected(t *testing.T) {
	// given
	got := []string{"adrs"}
	var expected []string

	// when
	err := domain.ValidateFilesRead(got, expected)

	// then
	if err != nil {
		t.Fatalf("expected nil error when no files expected, got %v", err)
	}
}

func TestValidateFilesRead_SupersetIsOK(t *testing.T) {
	// given: Claude read more files than expected (extra kinds are fine)
	got := []string{"adrs", "dods", "diff", "previous_scores", "pr_reviews"}
	expected := []string{"adrs", "dods", "diff", "previous_scores"}

	// when
	err := domain.ValidateFilesRead(got, expected)

	// then
	if err != nil {
		t.Fatalf("expected nil error for superset, got %v", err)
	}
}

func TestFormatEvalFile_ContainsYAMLFrontMatter(t *testing.T) {
	// given
	content := "# Architecture Decision Records\n\nSome ADR content here."

	// when
	result := domain.FormatEvalFile(domain.EvalKindADRs, content)

	// then
	if !strings.HasPrefix(result, "---\n") {
		t.Fatal("expected YAML front matter opening delimiter")
	}
	if !strings.Contains(result, "kind: adrs") {
		t.Fatal("expected kind field in front matter")
	}
	if !strings.Contains(result, "generated_by: amadeus") {
		t.Fatal("expected generated_by field in front matter")
	}
	if !strings.Contains(result, "read_only: true") {
		t.Fatal("expected read_only field in front matter")
	}
	if !strings.Contains(result, "warning:") {
		t.Fatal("expected warning field in front matter")
	}
	if !strings.Contains(result, content) {
		t.Fatal("expected original content preserved after front matter")
	}
}

func TestFormatEvalFile_FrontMatterEndDelimiter(t *testing.T) {
	// given
	content := "test content"

	// when
	result := domain.FormatEvalFile(domain.EvalKindDiff, content)

	// then: front matter has opening and closing ---
	parts := strings.SplitN(result, "---\n", 3)
	if len(parts) < 3 {
		t.Fatalf("expected opening and closing --- delimiters, got %d parts", len(parts))
	}
	// parts[0] is empty (before opening ---), parts[1] is front matter, parts[2] is body
	if !strings.Contains(parts[1], "kind: diff") {
		t.Fatalf("expected kind: diff in front matter, got %q", parts[1])
	}
	if !strings.Contains(parts[2], "test content") {
		t.Fatalf("expected body content after front matter, got %q", parts[2])
	}
}

func TestFormatEvalFile_EmptyContent(t *testing.T) {
	// given
	content := ""

	// when
	result := domain.FormatEvalFile(domain.EvalKindPreviousScores, content)

	// then: should still produce valid front matter with empty body
	if !strings.HasPrefix(result, "---\n") {
		t.Fatal("expected YAML front matter even with empty content")
	}
	if !strings.Contains(result, "kind: previous_scores") {
		t.Fatal("expected kind field")
	}
}
