package usecase

import (
	"fmt"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

// PrintSync orchestrates the sync status output.
// Validates the RunSyncCommand and delegates to session.
func PrintSync(cmd domain.RunSyncCommand, a *session.Amadeus) error {
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
	}
	return a.PrintSync()
}
