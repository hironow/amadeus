package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestRebuildCommand_RebuildsProjectionsFromEvents(t *testing.T) {
	// given: a temp .gate/ directory with events
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	eventsDir := filepath.Join(gateDir, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gateDir, ".run"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gateDir, "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gateDir, "outbox"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a CheckCompleted event to JSONL
	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
	ev, err := domain.NewEvent(domain.EventCheckCompleted, domain.CheckCompletedData{
		Result: amadeus.CheckResult{
			CheckedAt:  now,
			Commit:     "abc123",
			Type:       amadeus.CheckTypeFull,
			Divergence: 0.42,
		},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	line, _ := json.Marshal(ev)
	eventFile := filepath.Join(eventsDir, "2026-02-25.jsonl")
	if err := os.WriteFile(eventFile, append(line, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	// when: run rebuild command from the temp dir
	root := NewRootCommand()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"rebuild", "--config", filepath.Join(gateDir, "config.yaml")})

	// Override working directory for the test
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Ensure config exists
	if err := session.InitGateDir(gateDir); err != nil {
		t.Fatal(err)
	}

	err = root.Execute()

	// then: no error
	if err != nil {
		t.Fatalf("rebuild failed: %v", err)
	}

	// then: latest.json should be rebuilt from the event
	store := session.NewProjectionStore(gateDir)
	latest, err := store.LoadLatest()
	if err != nil {
		t.Fatalf("LoadLatest: %v", err)
	}
	if latest.Commit != "abc123" {
		t.Errorf("Commit = %q, want %q", latest.Commit, "abc123")
	}
	if latest.Divergence != 0.42 {
		t.Errorf("Divergence = %f, want %f", latest.Divergence, 0.42)
	}

	// then: stderr should contain summary
	output := stderr.String()
	if output == "" {
		t.Error("expected rebuild summary on stderr, got empty")
	}
}

func TestRebuildCommand_EmptyEventsSucceeds(t *testing.T) {
	// given: a temp .gate/ with empty events directory
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	if err := session.InitGateDir(gateDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gateDir, "events"), 0o755); err != nil {
		t.Fatal(err)
	}

	// when: run rebuild
	root := NewRootCommand()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"rebuild"})

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	err := root.Execute()

	// then: should succeed with 0 events
	if err != nil {
		t.Fatalf("rebuild with empty events failed: %v", err)
	}
}
