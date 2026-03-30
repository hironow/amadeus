package port

import (
	"context"
	"io"

	"github.com/hironow/amadeus/internal/domain"
)

// RunResult holds the output of a detailed runner invocation.
type RunResult struct {
	Text              string
	ProviderSessionID string
}

// DetailedRunner extends ClaudeRunner to also return session metadata.
type DetailedRunner interface {
	RunDetailed(ctx context.Context, prompt string, w io.Writer, opts ...RunOption) (RunResult, error)
}

// ListSessionOpts controls session listing filters.
type ListSessionOpts struct {
	Provider *domain.Provider
	Status   *domain.SessionStatus
	Limit    int
}

// CodingSessionStore persists and queries coding session records.
type CodingSessionStore interface {
	Save(ctx context.Context, record domain.CodingSessionRecord) error
	Load(ctx context.Context, id string) (domain.CodingSessionRecord, error)
	FindByProviderSessionID(ctx context.Context, provider domain.Provider, pid string) ([]domain.CodingSessionRecord, error)
	LatestByProviderSessionID(ctx context.Context, provider domain.Provider, pid string) (domain.CodingSessionRecord, error)
	List(ctx context.Context, opts ListSessionOpts) ([]domain.CodingSessionRecord, error)
	UpdateStatus(ctx context.Context, id string, status domain.SessionStatus, providerSessionID string, metadata map[string]string) error
	Close() error
}
