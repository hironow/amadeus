package domain

import (
	"time"
)

// ProviderErrorKind classifies the type of provider error.
type ProviderErrorKind int

const (
	// ProviderErrorNone indicates no provider-level error (normal failure).
	ProviderErrorNone ProviderErrorKind = iota
	// ProviderErrorRateLimit indicates a rate limit was hit.
	ProviderErrorRateLimit
	// ProviderErrorServer indicates a server-side error (5xx).
	ProviderErrorServer
)

// ProviderErrorInfo holds the classified result of a provider error.
type ProviderErrorInfo struct {
	Kind    ProviderErrorKind
	ResetAt time.Time // parsed reset time (zero if unknown)
}

// IsTrip returns true if the error should trip a circuit breaker.
func (i ProviderErrorInfo) IsTrip() bool {
	return i.Kind != ProviderErrorNone
}
