package session_test

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// fakeDetailedRunner implements port.DetailedRunner for testing.
type fakeDetailedRunner struct {
	sessionID string
	text      string
	err       error
}

func (f *fakeDetailedRunner) RunDetailed(_ context.Context, _ string, _ io.Writer, _ ...port.RunOption) (port.RunResult, error) {
	return port.RunResult{Text: f.text, ProviderSessionID: f.sessionID}, f.err
}

func TestSessionTrackingAdapter_RunSession_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	store, err := session.NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()

	adapter := session.NewSessionTrackingAdapter(
		&fakeDetailedRunner{sessionID: "claude-sess-xyz", text: "hello world"},
		store,
		domain.ProviderClaudeCode,
	)

	rec, text, err := adapter.RunSession(ctx, "do something", io.Discard)
	if err != nil {
		t.Fatalf("RunSession: %v", err)
	}

	if text != "hello world" {
		t.Errorf("text = %q, want %q", text, "hello world")
	}
	if rec.ProviderSessionID != "claude-sess-xyz" {
		t.Errorf("ProviderSessionID = %q, want %q", rec.ProviderSessionID, "claude-sess-xyz")
	}
	if rec.Status != domain.SessionCompleted {
		t.Errorf("Status = %q, want %q", rec.Status, domain.SessionCompleted)
	}

	// Verify persisted in store
	loaded, err := store.Load(ctx, rec.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Status != domain.SessionCompleted {
		t.Errorf("loaded Status = %q, want %q", loaded.Status, domain.SessionCompleted)
	}
	if loaded.ProviderSessionID != "claude-sess-xyz" {
		t.Errorf("loaded ProviderSessionID = %q, want %q", loaded.ProviderSessionID, "claude-sess-xyz")
	}
	if loaded.Metadata[domain.MetadataProviderState] != string(domain.ProviderStateActive) {
		t.Errorf("loaded Metadata[provider_state] = %q, want %q", loaded.Metadata[domain.MetadataProviderState], domain.ProviderStateActive)
	}
	if loaded.Metadata[domain.MetadataProviderRetryBudget] != "1" {
		t.Errorf("loaded Metadata[provider_retry_budget] = %q, want 1", loaded.Metadata[domain.MetadataProviderRetryBudget])
	}
}

func TestSessionTrackingAdapter_RunSession_Failure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	store, err := session.NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()

	adapter := session.NewSessionTrackingAdapter(
		&fakeDetailedRunner{sessionID: "sess-fail", text: "partial", err: context.DeadlineExceeded},
		store,
		domain.ProviderClaudeCode,
	)

	rec, text, err := adapter.RunSession(ctx, "do something", io.Discard)
	if err == nil {
		t.Fatal("RunSession should return error")
	}

	if text != "partial" {
		t.Errorf("text = %q, want %q", text, "partial")
	}
	if rec.Status != domain.SessionFailed {
		t.Errorf("Status = %q, want %q", rec.Status, domain.SessionFailed)
	}

	// Verify persisted as failed
	loaded, _ := store.Load(ctx, rec.ID)
	if loaded.Status != domain.SessionFailed {
		t.Errorf("loaded Status = %q, want %q", loaded.Status, domain.SessionFailed)
	}
	// Even on failure, provider session ID should be captured if available
	if loaded.ProviderSessionID != "sess-fail" {
		t.Errorf("loaded ProviderSessionID = %q, want %q", loaded.ProviderSessionID, "sess-fail")
	}
	if loaded.Metadata[domain.MetadataProviderState] != string(domain.ProviderStateActive) {
		t.Errorf("loaded Metadata[provider_state] = %q, want %q", loaded.Metadata[domain.MetadataProviderState], domain.ProviderStateActive)
	}
}

func TestSessionTrackingAdapter_RunSession_EmptyProviderSessionID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	store, err := session.NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()

	adapter := session.NewSessionTrackingAdapter(
		&fakeDetailedRunner{sessionID: "", text: "ok"},
		store,
		domain.ProviderClaudeCode,
	)

	rec, _, err := adapter.RunSession(ctx, "do something", io.Discard)
	if err != nil {
		t.Fatalf("RunSession: %v", err)
	}
	if rec.ProviderSessionID != "" {
		t.Errorf("ProviderSessionID should be empty, got %q", rec.ProviderSessionID)
	}
	if rec.Status != domain.SessionCompleted {
		t.Errorf("Status = %q, want %q", rec.Status, domain.SessionCompleted)
	}
}
