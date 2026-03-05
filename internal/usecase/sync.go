package usecase

import (
	"fmt"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// PrintSync orchestrates the sync status output.
// Validates the RunSyncCommand and delegates to session via port.Orchestrator.
func PrintSync(cmd domain.RunSyncCommand, pipeline port.Orchestrator) error {
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
	}
	return pipeline.PrintSync()
}
