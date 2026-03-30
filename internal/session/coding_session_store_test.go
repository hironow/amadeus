package session_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase/port"
)

func newTestSessionStore(t *testing.T) port.CodingSessionStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	store, err := session.NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteCodingSessionStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestCodingSessionStore_SaveAndLoad(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)
	ctx := context.Background()

	rec := domain.NewCodingSessionRecord(domain.ProviderClaudeCode, "opus", "/tmp/repo")

	if err := store.Save(ctx, rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(ctx, rec.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.ID != rec.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, rec.ID)
	}
	if loaded.Provider != domain.ProviderClaudeCode {
		t.Errorf("Provider = %q, want %q", loaded.Provider, domain.ProviderClaudeCode)
	}
	if loaded.Status != domain.SessionRunning {
		t.Errorf("Status = %q, want %q", loaded.Status, domain.SessionRunning)
	}
	if loaded.Model != "opus" {
		t.Errorf("Model = %q, want %q", loaded.Model, "opus")
	}
	if loaded.WorkDir != "/tmp/repo" {
		t.Errorf("WorkDir = %q, want %q", loaded.WorkDir, "/tmp/repo")
	}
}

func TestCodingSessionStore_UpdateStatus(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)
	ctx := context.Background()

	rec := domain.NewCodingSessionRecord(domain.ProviderClaudeCode, "opus", "/tmp/repo")
	_ = store.Save(ctx, rec)

	meta := map[string]string{"failure_reason": "timeout"}
	err := store.UpdateStatus(ctx, rec.ID, domain.SessionFailed, "provider-session-xyz", meta)
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	loaded, _ := store.Load(ctx, rec.ID)
	if loaded.Status != domain.SessionFailed {
		t.Errorf("Status = %q, want %q", loaded.Status, domain.SessionFailed)
	}
	if loaded.ProviderSessionID != "provider-session-xyz" {
		t.Errorf("ProviderSessionID = %q, want %q", loaded.ProviderSessionID, "provider-session-xyz")
	}
	if loaded.Metadata["failure_reason"] != "timeout" {
		t.Errorf("Metadata[failure_reason] = %q, want %q", loaded.Metadata["failure_reason"], "timeout")
	}
}

func TestCodingSessionStore_FindByProviderSessionID(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)
	ctx := context.Background()

	rec1 := domain.NewCodingSessionRecord(domain.ProviderClaudeCode, "opus", "/tmp/repo")
	_ = store.Save(ctx, rec1)
	_ = store.UpdateStatus(ctx, rec1.ID, domain.SessionCompleted, "prov-abc", nil)

	rec2 := domain.NewCodingSessionRecord(domain.ProviderClaudeCode, "opus", "/tmp/repo")
	_ = store.Save(ctx, rec2)
	_ = store.UpdateStatus(ctx, rec2.ID, domain.SessionCompleted, "prov-abc", nil)

	found, err := store.FindByProviderSessionID(ctx, domain.ProviderClaudeCode, "prov-abc")
	if err != nil {
		t.Fatalf("FindByProviderSessionID: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("expected 2 records, got %d", len(found))
	}

	// Different provider should not match
	found2, _ := store.FindByProviderSessionID(ctx, domain.ProviderCodex, "prov-abc")
	if len(found2) != 0 {
		t.Errorf("expected 0 records for different provider, got %d", len(found2))
	}
}

func TestCodingSessionStore_LatestByProviderSessionID(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)
	ctx := context.Background()

	rec1 := domain.NewCodingSessionRecord(domain.ProviderClaudeCode, "opus", "/tmp/repo-old")
	_ = store.Save(ctx, rec1)
	_ = store.UpdateStatus(ctx, rec1.ID, domain.SessionCompleted, "prov-abc", nil)

	rec2 := domain.NewCodingSessionRecord(domain.ProviderClaudeCode, "opus", "/tmp/repo-new")
	_ = store.Save(ctx, rec2)
	_ = store.UpdateStatus(ctx, rec2.ID, domain.SessionCompleted, "prov-abc", nil)

	latest, err := store.LatestByProviderSessionID(ctx, domain.ProviderClaudeCode, "prov-abc")
	if err != nil {
		t.Fatalf("LatestByProviderSessionID: %v", err)
	}
	if latest.ID != rec2.ID {
		t.Errorf("expected latest record ID %q, got %q", rec2.ID, latest.ID)
	}
}

func TestCodingSessionStore_List(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)
	ctx := context.Background()

	rec1 := domain.NewCodingSessionRecord(domain.ProviderClaudeCode, "opus", "/tmp/a")
	rec2 := domain.NewCodingSessionRecord(domain.ProviderCodex, "gpt-5", "/tmp/b")
	_ = store.Save(ctx, rec1)
	_ = store.Save(ctx, rec2)

	// List all
	all, err := store.List(ctx, port.ListSessionOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}

	// Filter by provider
	provider := domain.ProviderClaudeCode
	filtered, err := store.List(ctx, port.ListSessionOpts{Provider: &provider})
	if err != nil {
		t.Fatalf("List filtered: %v", err)
	}
	if len(filtered) != 1 {
		t.Errorf("expected 1, got %d", len(filtered))
	}

	// Limit
	limited, err := store.List(ctx, port.ListSessionOpts{Limit: 1})
	if err != nil {
		t.Fatalf("List limited: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("expected 1, got %d", len(limited))
	}
}

func TestCodingSessionStore_LoadNotFound(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)
	ctx := context.Background()

	_, err := store.Load(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Load should return error for nonexistent ID")
	}
}

func TestCodingSessionStore_SaveIdempotent(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)
	ctx := context.Background()

	rec := domain.NewCodingSessionRecord(domain.ProviderClaudeCode, "opus", "/tmp/repo")
	_ = store.Save(ctx, rec)

	// Second save with same ID should not error (idempotent)
	err := store.Save(ctx, rec)
	if err != nil {
		t.Fatalf("second Save should be idempotent, got: %v", err)
	}
}
