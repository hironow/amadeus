package port

import (
	"context"
	"io"

	"github.com/hironow/amadeus/internal/domain"
)

// RunResult holds the output of a detailed runner invocation.
type RunResult struct { // nosemgrep: structure.multiple-exported-structs-go,structure.exported-struct-and-interface-go -- coding session port family (RunResult/DetailedRunner/ListSessionOpts/CodingSessionStore) is a cohesive set for session tracking; RunResult co-locates with DetailedRunner as related return type [permanent]
	Text              string
	ProviderSessionID string
	Stderr            string // captured stderr for circuit breaker inspection
}

// DetailedRunner extends ProviderRunner to also return session metadata.
type DetailedRunner interface { // nosemgrep: structure.multiple-exported-interfaces-go -- coding session port family; see RunResult [permanent]
	RunDetailed(ctx context.Context, prompt string, w io.Writer, opts ...RunOption) (RunResult, error)
}

// ListSessionOpts controls session listing filters.
type ListSessionOpts struct { // nosemgrep: structure.multiple-exported-structs-go,structure.exported-struct-and-interface-go -- coding session port family; ListSessionOpts co-locates with CodingSessionStore as query parameter for the same port; see RunResult [permanent]
	Provider *domain.Provider
	Status   *domain.SessionStatus
	Limit    int
}

// CodingSessionStore persists and queries coding session records.
type CodingSessionStore interface { // nosemgrep: structure.multiple-exported-interfaces-go -- coding session port family; see RunResult [permanent]
	Save(ctx context.Context, record domain.CodingSessionRecord) error
	Load(ctx context.Context, id string) (domain.CodingSessionRecord, error)
	FindByProviderSessionID(ctx context.Context, provider domain.Provider, pid string) ([]domain.CodingSessionRecord, error)
	LatestByProviderSessionID(ctx context.Context, provider domain.Provider, pid string) (domain.CodingSessionRecord, error)
	List(ctx context.Context, opts ListSessionOpts) ([]domain.CodingSessionRecord, error)
	UpdateStatus(ctx context.Context, id string, status domain.SessionStatus, providerSessionID string, metadata map[string]string) error
	Close() error
}
