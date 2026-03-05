package session

import (
	"github.com/hironow/amadeus/internal/eventsource"
	"github.com/hironow/amadeus/internal/port"
)

// NewEventStore creates an event store for the given state directory.
// Derives the events path from the state root.
func NewEventStore(stateDir string) port.EventStore {
	return eventsource.NewFileEventStore(eventsource.EventsDir(stateDir))
}

// EventsDir returns the events directory path for a state root.
func EventsDir(stateDir string) string {
	return eventsource.EventsDir(stateDir)
}

// ListExpiredEventFiles returns .jsonl event file names older than the given days.
func ListExpiredEventFiles(stateDir string, days int) ([]string, error) {
	return eventsource.ListExpiredEventFiles(stateDir, days)
}

// PruneEventFiles deletes the named .jsonl files from the events directory.
func PruneEventFiles(stateDir string, files []string) ([]string, error) {
	return eventsource.PruneEventFiles(stateDir, files)
}
