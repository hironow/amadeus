package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/amadeus/internal/session"
)

func TestEnsureStateDir_CreatesCoreDirs(t *testing.T) {
	// given
	stateDir := filepath.Join(t.TempDir(), ".state")

	// when
	err := session.EnsureStateDir(stateDir)

	// then
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
}

func TestEnsureStateDir_WithMailDirs(t *testing.T) {
	// given
	stateDir := filepath.Join(t.TempDir(), ".state")

	// when
	err := session.EnsureStateDir(stateDir, session.WithMailDirs())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, sub := range []string{"inbox", "outbox", "archive"} {
		p := filepath.Join(stateDir, sub)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected mail dir %s to exist: %v", sub, err)
		}
	}
}

func TestEnsureStateDir_WithExtraDirs(t *testing.T) {
	// given
	stateDir := filepath.Join(t.TempDir(), ".state")

	// when
	err := session.EnsureStateDir(stateDir, session.WithExtraDirs("journal", "custom"))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, sub := range []string{"journal", "custom"} {
		p := filepath.Join(stateDir, sub)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected extra dir %s to exist: %v", sub, err)
		}
	}
}

func TestEnsureStateDir_Idempotent(t *testing.T) {
	// given
	stateDir := filepath.Join(t.TempDir(), ".state")

	// when: call twice
	if err := session.EnsureStateDir(stateDir, session.WithMailDirs()); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := session.EnsureStateDir(stateDir, session.WithMailDirs()); err != nil {
		t.Fatalf("second call: %v", err)
	}

	// then: no error, dirs still exist
	for _, sub := range []string{".run", "events", "insights", "inbox", "outbox", "archive"} {
		if _, err := os.Stat(filepath.Join(stateDir, sub)); err != nil {
			t.Errorf("expected %s to exist after idempotent call: %v", sub, err)
		}
	}
}
