package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/session"
)

func TestEnsureGitignoreEntries_NewFile(t *testing.T) {
	// given
	path := filepath.Join(t.TempDir(), ".gitignore")
	required := []string{"events/", ".run/", ".otel.env"}

	// when
	err := session.EnsureGitignoreEntries(path, required)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	for _, entry := range required {
		if !strings.Contains(content, entry) {
			t.Errorf("expected %q in gitignore, got: %s", entry, content)
		}
	}
}

func TestEnsureGitignoreEntries_AppendsMissing(t *testing.T) {
	// given: existing gitignore with one entry
	path := filepath.Join(t.TempDir(), ".gitignore")
	os.WriteFile(path, []byte("events/\n"), 0644)
	required := []string{"events/", ".run/", ".otel.env"}

	// when
	err := session.EnsureGitignoreEntries(path, required)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, ".run/") {
		t.Error("expected .run/ to be appended")
	}
	if !strings.Contains(content, ".otel.env") {
		t.Error("expected .otel.env to be appended")
	}
	// Original entry preserved
	if strings.Count(content, "events/") != 1 {
		t.Error("expected events/ to appear exactly once")
	}
}

func TestEnsureGitignoreEntries_AlreadyComplete(t *testing.T) {
	// given: all entries already present
	path := filepath.Join(t.TempDir(), ".gitignore")
	os.WriteFile(path, []byte("events/\n.run/\n.otel.env\n"), 0644)
	required := []string{"events/", ".run/", ".otel.env"}

	// when
	err := session.EnsureGitignoreEntries(path, required)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	// File should be unchanged
	if string(data) != "events/\n.run/\n.otel.env\n" {
		t.Errorf("file should be unchanged, got: %q", string(data))
	}
}

func TestEnsureGitignoreEntries_PreservesUserEntries(t *testing.T) {
	// given: user-added entries
	path := filepath.Join(t.TempDir(), ".gitignore")
	os.WriteFile(path, []byte("my-custom-dir/\n*.log\n"), 0644)
	required := []string{"events/", ".run/"}

	// when
	err := session.EnsureGitignoreEntries(path, required)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "my-custom-dir/") {
		t.Error("user entry should be preserved")
	}
	if !strings.Contains(content, "*.log") {
		t.Error("user entry should be preserved")
	}
}
