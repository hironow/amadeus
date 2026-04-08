package session

import (
	"context"
	"io"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// SessionTrackingAdapter wraps a DetailedRunner with provider-agnostic session persistence.
// It creates a CodingSessionRecord before each invocation, captures the
// provider's session ID from the result, updates the record in the store,
// and records circuit breaker state from stderr.
type SessionTrackingAdapter struct {
	inner    port.DetailedRunner
	store    port.CodingSessionStore
	provider domain.Provider
}

// NewSessionTrackingAdapter creates a new adapter.
func NewSessionTrackingAdapter(inner port.DetailedRunner, store port.CodingSessionStore, provider domain.Provider) *SessionTrackingAdapter {
	return &SessionTrackingAdapter{inner: inner, store: store, provider: provider}
}

// Run implements the provider runner interface, enabling drop-in replacement of plain adapters.
func (a *SessionTrackingAdapter) Run(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (string, error) {
	_, text, err := a.RunSession(ctx, prompt, w, opts...)
	return text, err
}

// RunSession executes the inner runner and persists session metadata.
func (a *SessionTrackingAdapter) RunSession(ctx context.Context, prompt string, w io.Writer, opts ...port.RunOption) (domain.CodingSessionRecord, string, error) {
	rc := port.ApplyOptions(opts...)
	rec := domain.NewCodingSessionRecord(a.provider, rc.Model, rc.WorkDir)

	// Persist running state (best-effort: if store fails, continue anyway)
	_ = a.store.Save(ctx, rec)

	// Inject CodingSessionID so stream events can be correlated with this record.
	opts = append(opts, port.WithCodingSessionID(rec.ID))
	result, runErr := a.inner.RunDetailed(ctx, prompt, w, opts...)

	// Circuit breaker: classify provider error from stderr
	recordCircuitBreaker(a.provider, runErr, result.Stderr)
	rec.Metadata = currentProviderState().ApplyMetadata(rec.Metadata)

	if runErr != nil {
		// Capture provider session ID even on failure
		rec.ProviderSessionID = result.ProviderSessionID
		rec.Status = domain.SessionFailed
		rec.Metadata["failure_reason"] = runErr.Error()
		if result.Stderr != "" {
			rec.Metadata["stderr"] = result.Stderr
		}
		_ = a.store.UpdateStatus(ctx, rec.ID, domain.SessionFailed, result.ProviderSessionID, rec.Metadata)
		return rec, result.Text, runErr
	}

	rec.ProviderSessionID = result.ProviderSessionID
	rec.Status = domain.SessionCompleted
	_ = a.store.UpdateStatus(ctx, rec.ID, domain.SessionCompleted, result.ProviderSessionID, rec.Metadata)

	return rec, result.Text, nil
}
