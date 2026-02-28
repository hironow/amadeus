package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArchivePrune_NegativeDays(t *testing.T) {
	// given
	root := NewRootCommand()
	root.SetArgs([]string{"archive-prune", "--days", "-5"})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for negative --days")
	}
	if !strings.Contains(err.Error(), "Days must be positive") {
		t.Errorf("expected 'Days must be positive' in error, got: %v", err)
	}
}

func TestArchivePrune_PrunesEventFiles(t *testing.T) {
	// given: create a temp dir with .gate/archive and .gate/events
	tmpDir := t.TempDir()
	archiveDir := filepath.Join(tmpDir, ".gate", "archive")
	eventsDir := filepath.Join(tmpDir, ".gate", "events")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create an old event file
	oldEventFile := filepath.Join(eventsDir, "2025-12-01.jsonl")
	if err := os.WriteFile(oldEventFile, []byte(`{"id":"1"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(oldEventFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Create a recent event file (should NOT be pruned)
	recentEventFile := filepath.Join(eventsDir, "2026-02-25.jsonl")
	if err := os.WriteFile(recentEventFile, []byte(`{"id":"2"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when: run archive-prune with --yes from the temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	root := NewRootCommand()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"archive-prune", "--days", "30", "--yes"})

	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Old event file should be deleted
	if _, statErr := os.Stat(oldEventFile); !os.IsNotExist(statErr) {
		t.Error("expected old event file to be deleted")
	}

	// Recent event file should remain
	if _, statErr := os.Stat(recentEventFile); statErr != nil {
		t.Error("expected recent event file to remain")
	}

	// Output should mention event files
	output := stderr.String()
	if !strings.Contains(output, "Event files") {
		t.Errorf("expected output to mention event files, got: %s", output)
	}
}

func TestArchivePrune_FailsWhenEventRecordFails(t *testing.T) {
	// given: archive with an old file, events dir is read-only so Append fails
	tmpDir := t.TempDir()
	archiveDir := filepath.Join(tmpDir, ".gate", "archive")
	eventsDir := filepath.Join(tmpDir, ".gate", "events")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create an old archive file that will be pruned
	oldFile := filepath.Join(archiveDir, "feedback-001.md")
	if err := os.WriteFile(oldFile, []byte("---\nname: feedback-001\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Make events dir read-only so event Append fails
	if err := os.Chmod(eventsDir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(eventsDir, 0o755) //nolint: restore for cleanup

	// when
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint: restore working dir

	root := NewRootCommand()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"archive-prune", "--days", "30", "--yes"})

	err := root.Execute()

	// then: command should return error because event recording failed
	if err == nil {
		t.Fatal("expected error when event recording fails, got nil")
	}
	if !strings.Contains(err.Error(), "archive.pruned event") {
		t.Errorf("expected 'archive.pruned event' in error, got: %v", err)
	}

	// Old file should still have been deleted (deletion happens before event recording)
	if _, statErr := os.Stat(oldFile); !os.IsNotExist(statErr) {
		t.Error("expected old archive file to be deleted despite event recording failure")
	}
}

func TestArchivePrune_ZeroDays(t *testing.T) {
	// given
	root := NewRootCommand()
	root.SetArgs([]string{"archive-prune", "--days", "0"})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for --days 0")
	}
	if !strings.Contains(err.Error(), "Days must be positive") {
		t.Errorf("expected 'Days must be positive' in error, got: %v", err)
	}
}
