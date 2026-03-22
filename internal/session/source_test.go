package session_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/session"
)

func TestCollectADRs_DirNotExist(t *testing.T) {
	// given: no docs/adr/ directory
	root := t.TempDir()

	// when
	result, err := session.CollectADRs(context.Background(), root)

	// then: returns empty string, no error
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestCollectADRs_EmptyDir(t *testing.T) {
	// given: docs/adr/ exists but has no .md files
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "adr"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// when
	result, err := session.CollectADRs(context.Background(), root)

	// then
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for empty dir, got %q", result)
	}
}

func TestCollectADRs_SingleFile(t *testing.T) {
	// given: one ADR file
	root := t.TempDir()
	adrDir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "0001-use-jwt.md"), []byte("Use JWT for auth"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// when
	result, err := session.CollectADRs(context.Background(), root)

	// then
	if err != nil {
		t.Fatalf("CollectADRs: %v", err)
	}
	if !strings.Contains(result, "### 0001-use-jwt.md") {
		t.Errorf("expected filename header, got %q", result)
	}
	if !strings.Contains(result, "Use JWT for auth") {
		t.Errorf("expected file content, got %q", result)
	}
}

func TestCollectADRs_MultipleFilesSorted(t *testing.T) {
	// given: multiple ADR files (should be sorted alphabetically)
	root := t.TempDir()
	adrDir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	for _, f := range []struct{ name, content string }{
		{"0002-use-grpc.md", "gRPC"},
		{"0001-use-rest.md", "REST"},
		{"0003-use-graphql.md", "GraphQL"},
	} {
		if err := os.WriteFile(filepath.Join(adrDir, f.name), []byte(f.content), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	// when
	result, err := session.CollectADRs(context.Background(), root)

	// then
	if err != nil {
		t.Fatalf("CollectADRs: %v", err)
	}
	// Verify order: 0001 before 0002 before 0003
	idx1 := strings.Index(result, "### 0001-use-rest.md")
	idx2 := strings.Index(result, "### 0002-use-grpc.md")
	idx3 := strings.Index(result, "### 0003-use-graphql.md")
	if idx1 < 0 || idx2 < 0 || idx3 < 0 {
		t.Fatalf("missing filename headers in result: %q", result)
	}
	if idx1 >= idx2 || idx2 >= idx3 {
		t.Errorf("files not in sorted order: idx1=%d, idx2=%d, idx3=%d", idx1, idx2, idx3)
	}
}

func TestCollectADRs_IgnoresNonMD(t *testing.T) {
	// given: mix of .md and non-.md files
	root := t.TempDir()
	adrDir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "0001-decision.md"), []byte("Decision"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "README.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(adrDir, "subdir"), 0o755); err != nil { // subdirectories should be ignored
		t.Fatalf("setup: %v", err)
	}

	// when
	result, err := session.CollectADRs(context.Background(), root)

	// then
	if err != nil {
		t.Fatalf("CollectADRs: %v", err)
	}
	if strings.Contains(result, "README.txt") {
		t.Error("should not include non-.md files")
	}
	if strings.Contains(result, "subdir") {
		t.Error("should not include subdirectories")
	}
	if !strings.Contains(result, "### 0001-decision.md") {
		t.Error("should include .md file")
	}
}

func TestCollectADRs_OutputFormat(t *testing.T) {
	// given: verify the exact output format used by LLM Judge prompt
	root := t.TempDir()
	adrDir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "0001-test.md"), []byte("content here"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// when
	result, err := session.CollectADRs(context.Background(), root)

	// then: format must be "### filename\ncontent" (no trailing newline)
	if err != nil {
		t.Fatalf("CollectADRs: %v", err)
	}
	expected := "### 0001-test.md\ncontent here"
	if result != expected {
		t.Errorf("output format mismatch:\ngot:  %q\nwant: %q", result, expected)
	}
}
