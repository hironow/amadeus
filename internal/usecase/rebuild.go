package usecase

import (
	"fmt"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// Rebuild replays events to regenerate projection files.
// The RebuildCommand is already valid by construction (parse-don't-validate).
func Rebuild(cmd domain.RebuildCommand, events port.EventStore, projector domain.EventApplier, logger domain.Logger) error {
	_ = cmd // command validated by construction; no fields accessed here

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
