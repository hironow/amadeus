package cmd
// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTargetDir_DefaultsToCwd(t *testing.T) {
	// given: no args
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}

	// when
	dir, resolveErr := resolveTargetDir(nil)

	// then
	if resolveErr != nil {
		t.Fatalf("resolveTargetDir failed: %v", resolveErr)
	}
	if dir != cwd {
		t.Errorf("expected cwd %q, got %q", cwd, dir)
	}
}

func TestResolveTargetDir_ValidPath(t *testing.T) {
	// given: valid directory path
	tmpDir := t.TempDir()

	// when
	dir, err := resolveTargetDir([]string{tmpDir})

	// then
	if err != nil {
		t.Fatalf("resolveTargetDir failed: %v", err)
	}
	abs, _ := filepath.Abs(tmpDir)
	if dir != abs {
		t.Errorf("expected %q, got %q", abs, dir)
	}
}

func TestResolveTargetDir_InvalidPath(t *testing.T) {
	// given: non-existent path
	badPath := filepath.Join(t.TempDir(), "nonexistent")

	// when
	_, err := resolveTargetDir([]string{badPath})

	// then
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
}

func TestResolveTargetDir_FileNotDir(t *testing.T) {
	// given: a file (not a directory)
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "somefile.txt")
	os.WriteFile(filePath, []byte("hello"), 0644)

	// when
	_, err := resolveTargetDir([]string{filePath})

	// then
	if err == nil {
		t.Fatal("expected error for file path")
	}
}
