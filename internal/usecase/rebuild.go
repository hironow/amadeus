package usecase

import (
	"fmt"

	amadeus "github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/domain"
)

// Rebuild replays events to regenerate projection files.
// Validates the RebuildCommand and performs the rebuild.
func Rebuild(cmd domain.RebuildCommand, events domain.EventStore, projector domain.EventApplier, logger *amadeus.Logger) error {
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
	}

	allEvents, err := events.LoadAll()
	if err != nil {
		return fmt.Errorf("load events: %w", err)
	}

	logger.Info("rebuilding projections from %d event(s)", len(allEvents))

	if err := projector.Rebuild(allEvents); err != nil {
		return fmt.Errorf("rebuild: %w", err)
	}

	logger.Info("rebuild complete")
	return nil
}
