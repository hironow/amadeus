package session

import (
	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness"
	"github.com/hironow/amadeus/internal/platform"
)

// sharedCircuitBreaker is the process-wide circuit breaker shared across all
// provider adapter instances. Set via SetCircuitBreaker at startup.
var sharedCircuitBreaker *platform.CircuitBreaker

// SetCircuitBreaker sets the process-wide circuit breaker for all provider calls.
// Call this once during startup before any provider invocations.
func SetCircuitBreaker(cb *platform.CircuitBreaker) {
	sharedCircuitBreaker = cb
}

// DivergenceMeterAllowedTools is the minimal tool set for divergence evaluation.
// The divergence meter only needs to read pre-collected content from the prompt;
// all filesystem I/O is done by Go before invoking Claude.
var DivergenceMeterAllowedTools = []string{
	"Read",
	"Bash(cat:*)",
}

// recordCircuitBreaker updates the shared circuit breaker based on provider error classification.
func recordCircuitBreaker(provider domain.Provider, err error, stderr string) {
	if sharedCircuitBreaker == nil {
		return
	}
	if err == nil {
		sharedCircuitBreaker.RecordSuccess()
		return
	}
	// Use stderr if available, otherwise try extracting from the error message itself
	classifyTarget := stderr
	if classifyTarget == "" {
		classifyTarget = err.Error()
	}
	info := harness.ClassifyProviderError(provider, classifyTarget)
	if info.IsTrip() {
		sharedCircuitBreaker.RecordProviderError(info)
	}
}

func currentProviderState() domain.ProviderStateSnapshot {
	if sharedCircuitBreaker == nil {
		return domain.ActiveProviderState()
	}
	return sharedCircuitBreaker.Snapshot()
}
