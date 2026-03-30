package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/session"
)

func TestTryLockDaemon_AcquiresLock(t *testing.T) {
	dir := t.TempDir()
	unlock, err := session.TryLockDaemon(dir)
	if err != nil {
		t.Fatalf("TryLockDaemon: %v", err)
	}
	if unlock == nil {
		t.Fatal("expected non-nil unlock function")
	}
	defer unlock()
}

func TestTryLockDaemon_RejectsSecondLock(t *testing.T) {
	dir := t.TempDir()
	unlock1, err := session.TryLockDaemon(dir)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	defer unlock1()

	_, err = session.TryLockDaemon(dir)
	if err == nil {
		t.Fatal("expected error for second lock attempt")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("expected 'already running' in error, got: %s", err.Error())
	}
}

func TestTryLockDaemon_ReleasesOnUnlock(t *testing.T) {
	dir := t.TempDir()
	unlock1, err := session.TryLockDaemon(dir)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	unlock1()

	unlock2, err := session.TryLockDaemon(dir)
	if err != nil {
		t.Fatalf("second lock after release: %v", err)
	}
	defer unlock2()
}

func TestTryLockDaemon_CreatesRunDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".run")
	unlock, err := session.TryLockDaemon(dir)
	if err != nil {
		t.Fatalf("TryLockDaemon: %v", err)
	}
	defer unlock()
	if _, statErr := os.Stat(filepath.Join(dir, "daemon.lock")); statErr != nil {
		t.Fatalf("expected daemon.lock to exist: %v", statErr)
	}
}
