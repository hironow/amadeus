package platform

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// ErrCircuitOpen is returned by Allow when the circuit breaker is open
// due to rate limiting or server errors.
var ErrCircuitOpen = errors.New("circuit breaker open: rate limit or server error")

// circuitState represents the state of the circuit breaker.
type circuitState int

const (
	circuitClosed  circuitState = iota // normal operation
	circuitOpen                        // blocking calls
	circuitHalfOpen                    // probing
)

// defaultBackoffBase is the initial wait duration when reset time is unknown.
const defaultBackoffBase = 30 * time.Second

// defaultBackoffMax caps exponential backoff.
const defaultBackoffMax = 10 * time.Minute

// CircuitBreaker prevents cascading failures when AI coding tool providers
// hit rate limits or server errors. Provider-agnostic: error classification
// is handled by domain.ClassifyProviderError before calling RecordProviderError.
type CircuitBreaker struct {
	mu             sync.Mutex
	state          circuitState
	resetAt        time.Time
	backoffCurrent time.Duration
	logger         domain.Logger
	tripped        int
	lastTrip       time.Time
}

// NewCircuitBreaker creates a circuit breaker in the closed state.
func NewCircuitBreaker(logger domain.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		state:          circuitClosed,
		backoffCurrent: defaultBackoffBase,
		logger:         logger,
	}
}

// Allow checks if a call is permitted. When the circuit is OPEN, it blocks
// until the reset time or backoff period elapses, then transitions to HALF_OPEN
// and returns nil. Returns context error if cancelled while waiting.
func (cb *CircuitBreaker) Allow(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		cb.mu.Lock()
		switch cb.state {
		case circuitClosed, circuitHalfOpen:
			cb.mu.Unlock()
			return nil
		case circuitOpen:
			// Check if reset time has passed
			if !cb.resetAt.IsZero() && time.Now().After(cb.resetAt) {
				cb.state = circuitHalfOpen
				cb.logger.Info("Circuit breaker: reset time reached, transitioning to HALF_OPEN (probe)")
				cb.mu.Unlock()
				return nil
			}
			// Check if backoff period has passed
			if cb.resetAt.IsZero() && time.Since(cb.lastTrip) > cb.backoffCurrent {
				cb.state = circuitHalfOpen
				cb.logger.Info("Circuit breaker: backoff elapsed, transitioning to HALF_OPEN (probe)")
				cb.mu.Unlock()
				return nil
			}

			// Calculate wait duration
			var waitDur time.Duration
			if !cb.resetAt.IsZero() {
				waitDur = time.Until(cb.resetAt)
				cb.logger.Warn("PAUSED — Provider rate limit reached. Resets at %s. Waiting...",
					cb.resetAt.Format("Jan 2, 3:04 PM (MST)"))
			} else {
				waitDur = cb.backoffCurrent - time.Since(cb.lastTrip)
				cb.logger.Warn("PAUSED — Provider server error. Waiting %v for recovery...", waitDur.Round(time.Second))
			}
			if waitDur <= 0 {
				waitDur = time.Second // minimum wait to avoid spin
			}
			cb.mu.Unlock()

			// Block until wait expires or context cancelled
			timer := time.NewTimer(waitDur)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
				// Loop back to re-check state
			}
		default:
			cb.mu.Unlock()
			return nil
		}
	}
}

// RecordProviderError updates the circuit breaker state based on a classified
// provider error. Callers should use domain.ClassifyProviderError to produce
// the ProviderErrorInfo before calling this method.
func (cb *CircuitBreaker) RecordProviderError(info domain.ProviderErrorInfo) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !info.IsTrip() {
		return
	}

	cb.state = circuitOpen
	cb.tripped++
	cb.lastTrip = time.Now()
	cb.backoffCurrent *= 2
	if cb.backoffCurrent > defaultBackoffMax {
		cb.backoffCurrent = defaultBackoffMax
	}
	cb.resetAt = info.ResetAt

	if !cb.resetAt.IsZero() {
		cb.logger.Warn("PAUSED — Circuit breaker OPEN. Rate limit resets at %s",
			cb.resetAt.Format("Jan 2, 3:04 PM (MST)"))
	} else {
		cb.logger.Warn("PAUSED — Circuit breaker OPEN. Server error detected, using backoff.")
	}
}

// RecordSuccess resets the circuit breaker to closed state.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state != circuitClosed {
		cb.logger.Info("Circuit breaker: CLOSED (recovered)")
		cb.backoffCurrent = defaultBackoffBase
	}
	cb.state = circuitClosed
	cb.resetAt = time.Time{}
}

// IsOpen returns true if the circuit breaker is in the open state.
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state == circuitOpen
}

// ResetAt returns the parsed reset time (zero if unknown).
func (cb *CircuitBreaker) ResetAt() time.Time {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.resetAt
}

// String returns a human-readable state description.
func (cb *CircuitBreaker) String() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case circuitClosed:
		return "CLOSED"
	case circuitOpen:
		if !cb.resetAt.IsZero() {
			return fmt.Sprintf("OPEN (resets at %s)", cb.resetAt.Format("15:04 MST"))
		}
		return "OPEN (backoff)"
	case circuitHalfOpen:
		return "HALF_OPEN"
	}
	return "UNKNOWN"
}
