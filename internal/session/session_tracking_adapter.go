package session

import (
	"context"
	"io"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// SessionTrackingAdapter wraps a DetailedRunner with session persistence.
// It creates a CodingSessionRecord before each invocation, captures the
// provider session ID from the result, and updates the record in the store.
type SessionTrackingAdapter struct {
	inner    port.DetailedRunner
	store    port.CodingSessionStore
	provider domain.Provider
}

// NewSessionTrackingAdapter creates a new adapter.
func NewSessionTrackingAdapter(inner port.DetailedRunner, store port.CodingSessionStore, provider domain.Provider) *SessionTrackingAdapter {
	return &SessionTrackingAdapter{inner: inner, store: store, provider: provider}
}

// RunSession executes the inner runner and persists session metadata.
func (a *SessionTrackingAdapter) RunSession(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (domain.CodingSessionRecord, string, error) {
	rc := port.ApplyOptions(opts...)
	rec := domain.NewCodingSessionRecord(a.provider, "", rc.WorkDir)

	// Persist running state (best-effort: if store fails, continue anyway)
	_ = a.store.Save(ctx, rec)

	result, runErr := a.inner.RunDetailed(ctx, prompt, w, opts...)

	if runErr != nil {
		// Capture provider session ID even on failure
		rec.ProviderSessionID = result.ProviderSessionID
		rec.Status = domain.SessionFailed
		if rec.Metadata == nil {
			rec.Metadata = make(map[string]string)
		}
		rec.Metadata["failure_reason"] = runErr.Error()
		_ = a.store.UpdateStatus(ctx, rec.ID, domain.SessionFailed, result.ProviderSessionID)
		return rec, result.Text, runErr
	}

	rec.ProviderSessionID = result.ProviderSessionID
	rec.Status = domain.SessionCompleted
	_ = a.store.UpdateStatus(ctx, rec.ID, domain.SessionCompleted, result.ProviderSessionID)

	return rec, result.Text, nil
}
