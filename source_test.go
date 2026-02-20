package amadeus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectMarkdownFiles_DirNotExist(t *testing.T) {
	// given
	dir := filepath.Join(t.TempDir(), "nonexistent")

	// when
	result, err := collectMarkdownFiles(dir)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestCollectMarkdownFiles_EmptyDir(t *testing.T) {
	// given
	dir := t.TempDir()

	// when
	result, err := collectMarkdownFiles(dir)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestCollectMarkdownFiles_SingleFile(t *testing.T) {
	// given
	dir := t.TempDir()
	content := "# 0001. Use Go\n\nWe decided to use Go.\n"
	if err := os.WriteFile(filepath.Join(dir, "0001-use-go.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	result, err := collectMarkdownFiles(dir)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(result, "### 0001-use-go.md") {
		t.Errorf("expected filename header, got %q", result)
	}
	if !strings.Contains(result, "We decided to use Go.") {
		t.Errorf("expected file content, got %q", result)
	}
}

func TestCollectMarkdownFiles_MultipleFilesSortedByName(t *testing.T) {
	// given
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "0002-jwt-auth.md"), []byte("# JWT Auth\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "0001-use-go.md"), []byte("# Use Go\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	result, err := collectMarkdownFiles(dir)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	idx1 := strings.Index(result, "0001-use-go.md")
	idx2 := strings.Index(result, "0002-jwt-auth.md")
	if idx1 < 0 || idx2 < 0 {
		t.Fatalf("expected both files in output, got %q", result)
	}
	if idx1 >= idx2 {
		t.Errorf("expected 0001 before 0002, got 0001 at %d, 0002 at %d", idx1, idx2)
	}
}

func TestCollectMarkdownFiles_IgnoresNonMdFiles(t *testing.T) {
	// given
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "0001-use-go.md"), []byte("# Use Go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("some notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	result, err := collectMarkdownFiles(dir)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(result, "0001-use-go.md") {
		t.Errorf("expected .md file in output, got %q", result)
	}
	if strings.Contains(result, "notes.txt") {
		t.Errorf("expected .txt file to be ignored, got %q", result)
	}
}

func TestCollectADRs_DelegatesToCorrectPath(t *testing.T) {
	// given
	repoRoot := t.TempDir()
	adrDir := filepath.Join(repoRoot, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "0001-test.md"), []byte("# Test ADR\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	result, err := CollectADRs(repoRoot)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(result, "Test ADR") {
		t.Errorf("expected ADR content, got %q", result)
	}
}

func TestCollectADRs_MissingDir_GracefulDegradation(t *testing.T) {
	// given
	repoRoot := t.TempDir()

	// when
	result, err := CollectADRs(repoRoot)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestCollectDoDs_WithFiles(t *testing.T) {
	// given
	repoRoot := t.TempDir()
	dodDir := filepath.Join(repoRoot, "docs", "dod")
	if err := os.MkdirAll(dodDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dodDir, "sprint-42.md"), []byte("# Sprint 42 DoD\n\n- All tests pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	result, err := CollectDoDs(repoRoot)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(result, "Sprint 42 DoD") {
		t.Errorf("expected DoD content, got %q", result)
	}
}

func TestCollectDoDs_MissingDir_GracefulDegradation(t *testing.T) {
	// given
	repoRoot := t.TempDir()

	// when
	result, err := CollectDoDs(repoRoot)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestCollectDependencyMap_ValidGoMod(t *testing.T) {
	// given
	repoRoot := t.TempDir()
	gomod := `module example.com/myapp

go 1.25.0

require (
	github.com/foo/bar v1.2.3
	github.com/baz/qux v0.1.0
)
`
	if err := os.WriteFile(filepath.Join(repoRoot, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	result, err := CollectDependencyMap(repoRoot)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(result, "github.com/foo/bar") {
		t.Errorf("expected dependency in output, got %q", result)
	}
	if !strings.Contains(result, "go 1.25.0") {
		t.Errorf("expected go version in output, got %q", result)
	}
}

func TestCollectDependencyMap_MissingGoMod(t *testing.T) {
	// given
	repoRoot := t.TempDir()

	// when
	result, err := CollectDependencyMap(repoRoot)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestCollectDependencyMap_EmptyGoMod(t *testing.T) {
	// given
	repoRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoRoot, "go.mod"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	result, err := CollectDependencyMap(repoRoot)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}
