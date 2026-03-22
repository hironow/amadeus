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

	allEvents, _, err := events.LoadAll()
	if err != nil {
		return fmt.Errorf("load events: %w", err)
	}

	// #116: Trim check history before event replay to prevent unbounded growth.
	trimmed, dropped := domain.TrimCheckHistory(allEvents, domain.DefaultMaxResultHistory)
	if dropped > 0 {
		logger.Info("trimmed %d old check event(s) before replay", dropped)
	}

	logger.Info("rebuilding projections from %d event(s)", len(trimmed))

	if err := projector.Rebuild(trimmed); err != nil {
		return fmt.Errorf("rebuild: %w", err)
	}

	logger.Info("rebuild complete")
	return nil
}
