package domain

import "strings"

// StopCategory classifies the reason a run stopped for operational safety decisions.
type StopCategory string

const (
	// StopGraceful indicates the run ended normally (e.g., context canceled, normal exit).
	StopGraceful StopCategory = "graceful"
	// StopUser indicates the run was terminated by a user action (e.g., signal).
	StopUser StopCategory = "user"
	// StopIOError indicates the run stopped due to an IO failure (e.g., read/write/EOF error).
	StopIOError StopCategory = "io_error"
	// StopTransient indicates the run stopped due to a transient condition (e.g., timeout, connection refused).
	StopTransient StopCategory = "transient"
	// StopUnknown is the safe fallback for unrecognized reasons.
	StopUnknown StopCategory = "unknown"
)

// ClassifyStopReason classifies a RunStoppedData.Reason string into a StopCategory.
// Pattern ordering is critical: graceful is checked before transient to ensure
// "context deadline exceeded" maps to StopGraceful, not StopTransient.
func ClassifyStopReason(reason string) StopCategory {
	lower := strings.ToLower(reason)

	// Graceful: normal exit conditions including context cancellation and deadline
	if lower == "" ||
		strings.Contains(lower, "context canceled") ||
		strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "normal exit") {
		return StopGraceful
	}

	// User: signal-based or explicit user-triggered termination
	if strings.Contains(lower, "signal") ||
		strings.Contains(lower, "sig") ||
		strings.Contains(lower, "user") {
		return StopUser
	}

	// IO error: read, write, or EOF failures
	if strings.Contains(lower, "read") ||
		strings.Contains(lower, "write") ||
		strings.Contains(lower, "eof") {
		return StopIOError
	}

	// Transient: timeout or connectivity issues
	if strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "connection") ||
		strings.Contains(lower, "temporary") {
		return StopTransient
	}

	return StopUnknown
}

// IsCriticalStop returns true only when the category indicates an IO error,
// which requires immediate operational attention.
func IsCriticalStop(cat StopCategory) bool {
	return cat == StopIOError
}
