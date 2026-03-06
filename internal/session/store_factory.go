package session

import (
	"context"

	"go.opentelemetry.io/otel/attribute"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/eventsource"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// NewEventStore creates an event store for the given state directory.
// eventsource is the event persistence adapter (AWS Event Sourcing pattern).
// Derives the events path from the state root.
func NewEventStore(stateDir string, logger domain.Logger) port.EventStore {
	raw := eventsource.NewFileEventStore(eventsource.EventsDir(stateDir), logger)
	return NewSpanEventStore(raw)
}

// EventsDir returns the events directory path for a state root.
func EventsDir(stateDir string) string {
	return eventsource.EventsDir(stateDir)
}

// ListExpiredEventFiles returns .jsonl event file names older than the given days.
// cmd layer should use this instead of importing eventsource directly (ADR S0008).
func ListExpiredEventFiles(ctx context.Context, stateDir string, days int) ([]string, error) {
	_, span := platform.Tracer.Start(ctx, "eventsource.list_expired")
	defer span.End()

	files, err := eventsource.ListExpiredEventFiles(stateDir, days)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "eventsource.list_expired"))
	}
	span.SetAttributes(attribute.Int("event.count.out", len(files)))
	return files, err
}

// PruneEventFiles deletes the named .jsonl files from the events directory.
// cmd layer should use this instead of importing eventsource directly (ADR S0008).
func PruneEventFiles(ctx context.Context, stateDir string, files []string) ([]string, error) {
	_, span := platform.Tracer.Start(ctx, "eventsource.prune")
	defer span.End()

	span.SetAttributes(attribute.Int("event.count.in", len(files)))
	deleted, err := eventsource.PruneEventFiles(stateDir, files)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "eventsource.prune"))
	}
	span.SetAttributes(attribute.Int("event.count.out", len(deleted)))
	return deleted, err
}
