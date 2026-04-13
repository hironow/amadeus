package session_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/session"
)

func TestEnsureStateDir_CreatesCoreDirs(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), ".state")

	result, err := session.EnsureStateDir(stateDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, sub := range []string{"", ".run", "events", "insights"} {
		p := filepath.Join(stateDir, sub)
		if info, err := os.Stat(p); err != nil {
			t.Errorf("expected dir %s to exist: %v", sub, err)
		} else if !info.IsDir() {
			t.Errorf("expected %s to be a directory", sub)
		}
	}
	if result.StateDir != ".state" {
		t.Errorf("StateDir = %q, want .state", result.StateDir)
	}
}

func TestEnsureStateDir_WithMailDirs(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), ".state")

	result, err := session.EnsureStateDir(stateDir, session.WithMailDirs())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, sub := range []string{"inbox", "outbox", "archive"} {
		p := filepath.Join(stateDir, sub)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected mail dir %s to exist: %v", sub, err)
		}
	}
	// Should have created entries
	createdCount := 0
	for _, e := range result.Entries {
		if e.Action == session.InitCreated {
			createdCount++
		}
	}
	if createdCount < 7 { // state + .run + events + insights + inbox + outbox + archive
		t.Errorf("expected at least 7 created entries, got %d", createdCount)
	}
}

func TestEnsureStateDir_WithExtraDirs(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), ".state")

	_, err := session.EnsureStateDir(stateDir, session.WithExtraDirs("journal", "custom"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, sub := range []string{"journal", "custom"} {
		if _, err := os.Stat(filepath.Join(stateDir, sub)); err != nil {
			t.Errorf("expected extra dir %s to exist: %v", sub, err)
		}
	}
}

func TestEnsureStateDir_Idempotent_RecordsSkipped(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), ".state")

	// First call: all created
	r1, _ := session.EnsureStateDir(stateDir, session.WithMailDirs())
	createdFirst := 0
	for _, e := range r1.Entries {
		if e.Action == session.InitCreated {
			createdFirst++
		}
	}

	// Second call: all skipped
	r2, err := session.EnsureStateDir(stateDir, session.WithMailDirs())
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	skippedSecond := 0
	for _, e := range r2.Entries {
		if e.Action == session.InitSkipped {
			skippedSecond++
		}
	}

	if skippedSecond != createdFirst {
		t.Errorf("second call: %d skipped, want %d (same as first created)", skippedSecond, createdFirst)
	}
}

func TestPrintInitResult(t *testing.T) {
	result := &session.InitResult{StateDir: ".siren"}
	result.Add(".siren/", session.InitCreated, "")
	result.Add(".siren/.run/", session.InitCreated, "")
	result.Add(".siren/config.yaml", session.InitUpdated, "")
	result.Add("skills", session.InitWarning, "failed to install")

	var buf bytes.Buffer
	session.PrintInitResult(&buf, result)

	output := buf.String()
	if !strings.Contains(output, "Initialized .siren/") {
		t.Error("missing header")
	}
	if !strings.Contains(output, "+ .siren/") {
		t.Error("missing created entry")
	}
	if !strings.Contains(output, "~ .siren/config.yaml") {
		t.Error("missing updated entry")
	}
	if !strings.Contains(output, "! failed to install") {
		t.Error("missing warning entry")
	}
}
