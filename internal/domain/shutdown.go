package domain

// shutdownKey is the context key for the outer (shutdown) context.
type shutdownKey struct{}

// ShutdownKey is used to embed the outer context in workCtx via context.WithValue.
// Commands retrieve it to get a context that survives workCtx cancellation.
var ShutdownKey = shutdownKey{}
