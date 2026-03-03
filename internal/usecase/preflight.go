package usecase

import "github.com/hironow/amadeus/internal/session"

// PreflightCheck verifies that required binaries are available in PATH.
func PreflightCheck(binaries ...string) error {
	return session.PreflightCheck(binaries...)
}
