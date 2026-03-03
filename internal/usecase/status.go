package usecase

import (
	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

// GetStatus collects and returns the operational status report for a gate directory.
func GetStatus(gateDir string) domain.StatusReport {
	return session.Status(gateDir)
}
