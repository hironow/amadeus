package session_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/session"
)

func TestSQLiteRunLockStore_TryAcquireAndRelease(t *testing.T) {
	// given
	stateDir := t.TempDir()
	dbPath := filepath.Join(stateDir, ".run", "run_locks.db")
	store, err := session.NewSQLiteRunLockStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// when — acquire a lock
	acquired, holder, err := store.TryAcquire(ctx, "wave-1", 30*time.Minute)

	// then
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	if !acquired {
		t.Errorf("expected acquired=true, got false (holder=%s)", holder)
	}

	// when — try to acquire again with different holder
	acquired2, holder2, err := store.TryAcquire(ctx, "wave-1", 30*time.Minute)

	// then — should fail because already held
	if err != nil {
		t.Fatalf("TryAcquire second: %v", err)
	}
	if acquired2 {
		t.Error("expected acquired=false for second attempt")
	}
	if holder2 == "" {
		t.Error("expected non-empty holder for second attempt")
	}

	// when — check IsHeld
	held, heldBy, err := store.IsHeld(ctx, "wave-1")
	if err != nil {
		t.Fatalf("IsHeld: %v", err)
	}
	if !held {
		t.Error("expected held=true")
	}
	if heldBy == "" {
		t.Error("expected non-empty holder from IsHeld")
	}

	// when — release
	err = store.Release(ctx, "wave-1", heldBy)
	if err != nil {
		t.Fatalf("Release: %v", err)
	}

	// then — should be acquirable again
	acquired3, _, err := store.TryAcquire(ctx, "wave-1", 30*time.Minute)
	if err != nil {
		t.Fatalf("TryAcquire after release: %v", err)
	}
	if !acquired3 {
		t.Error("expected acquired=true after release")
	}
}

func TestSQLiteRunLockStore_StaleLockCleanup(t *testing.T) {
	// given — a lock with very short TTL
	stateDir := t.TempDir()
	dbPath := filepath.Join(stateDir, ".run", "run_locks.db")
	store, err := session.NewSQLiteRunLockStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// acquire with 1ms TTL (will expire immediately)
	_, _, err = store.TryAcquire(ctx, "stale-key", 1*time.Millisecond)
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}

	// wait for expiry
	time.Sleep(5 * time.Millisecond)

	// when — try to acquire the same key (should succeed due to stale cleanup)
	acquired, _, err := store.TryAcquire(ctx, "stale-key", 30*time.Minute)

	// then
	if err != nil {
		t.Fatalf("TryAcquire after stale: %v", err)
	}
	if !acquired {
		t.Error("expected acquired=true after stale lock expired")
	}
}

func TestSQLiteRunLockStore_IsHeld_NotHeld(t *testing.T) {
	// given
	stateDir := t.TempDir()
	dbPath := filepath.Join(stateDir, ".run", "run_locks.db")
	store, err := session.NewSQLiteRunLockStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	// when
	held, holder, err := store.IsHeld(context.Background(), "nonexistent")

	// then
	if err != nil {
		t.Fatalf("IsHeld: %v", err)
	}
	if held {
		t.Error("expected held=false for nonexistent key")
	}
	if holder != "" {
		t.Errorf("expected empty holder, got %q", holder)
	}
}
