package amadeus

import "time"

// EventStore is the append-only event persistence interface.
type EventStore interface {
	// Append persists one or more events atomically.
	Append(events ...Event) error

	// LoadAll returns all events in chronological order.
	LoadAll() ([]Event, error)

	// LoadSince returns events with timestamps after the given time.
	LoadSince(after time.Time) ([]Event, error)
}
