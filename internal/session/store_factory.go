package session

import (
	"time"

	"github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/eventsource"
)

// ExpiredEventFile re-exports eventsource.ExpiredFile so cmd layer
// does not need to import eventsource directly (ADR S0008).
type ExpiredEventFile = eventsource.ExpiredFile

// NewEventStore creates an event store for the given gate directory.
// Derives the events path from the gate root.
func NewEventStore(gateDir string) amadeus.EventStore {
	return eventsource.NewFileEventStore(eventsource.EventsDir(gateDir))
}

// NewEventStoreFromEventsDir creates an event store from an explicit events directory path.
func NewEventStoreFromEventsDir(eventsDir string) amadeus.EventStore {
	return eventsource.NewFileEventStore(eventsDir)
}

// EventsDir returns the events directory path for a gate root.
func EventsDir(gateDir string) string {
	return eventsource.EventsDir(gateDir)
}

// FindExpiredEventFiles returns .jsonl event files older than maxAge.
func FindExpiredEventFiles(eventsDir string, maxAge time.Duration) ([]ExpiredEventFile, error) {
	return eventsource.FindExpiredEventFiles(eventsDir, maxAge)
}

// PruneEventFiles deletes the specified expired event files.
func PruneEventFiles(files []ExpiredEventFile) (int, error) {
	return eventsource.PruneEventFiles(files)
}
