package domain

import "time"

// shutdownKey is the context key for the outer (shutdown) context.
type shutdownKey struct{}

// ShutdownKey is used to embed the outer context in workCtx via context.WithValue.
// Commands retrieve it to get a context that survives workCtx cancellation.
var ShutdownKey = shutdownKey{}

// IndexEntry represents one line in the archive index JSONL file.
type IndexEntry struct {
	Timestamp string `json:"ts"`
	Operation string `json:"op"`
	Issue     string `json:"issue"`
	Status    string `json:"status"`
	Tool      string `json:"tool"`
	Path      string `json:"path"`
	Summary   string `json:"summary"`
}

// HandoverState captures in-progress work state when an operation is
// interrupted by a signal. The struct is pure data — no context, no I/O.
type HandoverState struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go — pure signal-interrupt data carrier; Completed/Remaining are transient in-flight state lists, not domain invariant collections [permanent]
	Tool         string // "amadeus"
	Operation    string // "divergence"
	Timestamp    time.Time
	InProgress   string            // Current task description
	Completed    []string          // What was done
	Remaining    []string          // What's left
	PartialState map[string]string // Tool-specific state (key=label, value=detail)
}

// MergedPR represents a merged pull request.
type MergedPR struct {
	Number string
	Title  string
}

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
