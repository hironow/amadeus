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

// PrintSyncFromParams constructs an Amadeus from AmadeusParams and runs the sync pipeline.
// This is the cmd-facing entry point that eliminates session imports from cmd.
func PrintSyncFromParams(cmd domain.RunSyncCommand, params AmadeusParams) error {
	result, err := buildAmadeus(params)
	if err != nil {
		return fmt.Errorf("build amadeus: %w", err)
	}
	defer result.Cleanup()

	return PrintSync(cmd, result.Amadeus)
}
