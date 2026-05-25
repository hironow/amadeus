package port

import (
	"context"

	"github.com/hironow/amadeus/internal/domain"
)

// ListSessionOpts controls session listing filters.
type ListSessionOpts struct { // nosemgrep: structure.multiple-exported-structs-go,structure.exported-struct-and-interface-go -- coding session port family; ListSessionOpts co-locates with CodingSessionStore as query parameter for the same port [permanent]
	Provider *domain.Provider
	Status   *domain.SessionStatus
	Limit    int
}

// CodingSessionStore persists and queries coding session records.
type CodingSessionStore interface { // nosemgrep: structure.multiple-exported-interfaces-go -- coding session port family; co-locates with ListSessionOpts as the store query surface [permanent]
	Save(ctx context.Context, record domain.CodingSessionRecord) error
	Load(ctx context.Context, id string) (domain.CodingSessionRecord, error)
	FindByProviderSessionID(ctx context.Context, provider domain.Provider, pid string) ([]domain.CodingSessionRecord, error)
	LatestByProviderSessionID(ctx context.Context, provider domain.Provider, pid string) (domain.CodingSessionRecord, error)
	List(ctx context.Context, opts ListSessionOpts) ([]domain.CodingSessionRecord, error)
	UpdateStatus(ctx context.Context, id string, status domain.SessionStatus, providerSessionID string, metadata map[string]string) error
	Close() error
}
