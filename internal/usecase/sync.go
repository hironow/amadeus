package usecase

import (
	"fmt"

	amadeus "github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/session"
)

// PrintSync orchestrates the sync status output.
// Validates the RunSyncCommand and delegates to session.
func PrintSync(cmd amadeus.RunSyncCommand, a *session.Amadeus) error {
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
	}
	return a.PrintSync()
}
