package usecase

import (
	"fmt"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

// Rebuild replays events to regenerate projection files.
// Validates the RebuildCommand and performs the rebuild.
func Rebuild(cmd domain.RebuildCommand, events domain.EventStore, projector domain.EventApplier, logger domain.Logger) error {
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

// RebuildFromDir constructs event store, projection store, and outbox store
// from a gate directory, then replays events to regenerate projections.
// This is the cmd-facing entry point that eliminates session imports from cmd.
func RebuildFromDir(cmd domain.RebuildCommand, gateDir string, logger domain.Logger) error {
	eventStore := session.NewEventStore(gateDir)
	store := session.NewProjectionStore(gateDir)

	outboxStore, err := session.NewOutboxStoreForDir(gateDir)
	if err != nil {
		return fmt.Errorf("outbox store: %w", err)
	}
	defer outboxStore.Close()

	projector := &session.Projector{Store: store, OutboxStore: outboxStore}

	return Rebuild(cmd, eventStore, projector, logger)
}
