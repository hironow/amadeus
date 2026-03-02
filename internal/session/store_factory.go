package session

import (
	"github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/eventsource"
)

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

// ListExpiredEventFiles returns .jsonl event file names older than the given days.
func ListExpiredEventFiles(stateDir string, days int) ([]string, error) {
	return eventsource.ListExpiredEventFiles(stateDir, days)
}

// PruneEventFiles deletes the named .jsonl files from the events directory.
func PruneEventFiles(stateDir string, files []string) ([]string, error) {
	return eventsource.PruneEventFiles(stateDir, files)
}
