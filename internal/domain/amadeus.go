package domain

import (
	"errors"
	"fmt"
)

// DriftError is returned by RunCheck when drift is detected (D-Mails generated).
// Callers can use errors.As to distinguish drift from runtime errors.
type DriftError struct {
	Divergence float64
	DMails     int
}

func (e *DriftError) Error() string {
	return fmt.Sprintf("drift detected: divergence=%f, %d D-Mail(s)", e.Divergence, e.DMails)
}

// ExitCode maps an error to a process exit code.
//
//	nil        → 0 (success)
//	DriftError → 2 (drift detected)
//	other      → 1 (runtime error)
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var de *DriftError
	if errors.As(err, &de) {
		return 2
	}
	return 1
}

// CheckOptions controls how RunCheck operates.
type CheckOptions struct {
	Full   bool
	DryRun bool
	Quiet  bool
	JSON   bool
}
