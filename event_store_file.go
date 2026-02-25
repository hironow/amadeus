package amadeus

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// FileEventStore implements EventStore using daily JSONL files in a directory.
// Each file is named YYYY-MM-DD.jsonl and contains one JSON event per line.
type FileEventStore struct {
	Dir string
}

// Append persists events as JSONL lines to the daily file based on each event's timestamp.
// All events are validated before any writes occur; if any event is invalid, the entire batch is rejected.
func (s *FileEventStore) Append(events ...Event) error {
	for _, ev := range events {
		if err := ValidateEvent(ev); err != nil {
			return fmt.Errorf("validate event %s: %w", ev.ID, err)
		}
	}

	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("create event store dir: %w", err)
	}

	// Group events by date for file routing
	byDate := make(map[string][]Event)
	for _, ev := range events {
		date := ev.Timestamp.Format("2006-01-02")
		byDate[date] = append(byDate[date], ev)
	}

	for date, evs := range byDate {
		filename := filepath.Join(s.Dir, date+".jsonl")
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("open event file %s: %w", date, err)
		}
		for _, ev := range evs {
			line, err := json.Marshal(ev)
			if err != nil {
				f.Close()
				return fmt.Errorf("marshal event %s: %w", ev.ID, err)
			}
			if _, err := f.Write(append(line, '\n')); err != nil {
				f.Close()
				return fmt.Errorf("write event %s: %w", ev.ID, err)
			}
		}
		if err := f.Sync(); err != nil {
			f.Close()
			return fmt.Errorf("fsync event file %s: %w", date, err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close event file %s: %w", date, err)
		}
	}
	return nil
}

// LoadAll reads all JSONL files in lexicographic order and returns events chronologically.
func (s *FileEventStore) LoadAll() ([]Event, error) {
	return s.loadEvents(time.Time{})
}

// LoadSince returns events with timestamps strictly after the given time.
func (s *FileEventStore) LoadSince(after time.Time) ([]Event, error) {
	return s.loadEvents(after)
}

func (s *FileEventStore) loadEvents(after time.Time) ([]Event, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read event store dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	var events []Event
	for _, name := range files {
		path := filepath.Join(s.Dir, name)
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", name, err)
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // allow up to 10MiB per line
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var ev Event
			if err := json.Unmarshal(line, &ev); err != nil {
				f.Close()
				return nil, fmt.Errorf("parse event in %s: %w", name, err)
			}
			if !after.IsZero() && !ev.Timestamp.After(after) {
				continue
			}
			events = append(events, ev)
		}
		if err := scanner.Err(); err != nil {
			f.Close()
			return nil, fmt.Errorf("scan %s: %w", name, err)
		}
		f.Close()
	}

	// Stable sort preserves insertion order for events with equal timestamps.
	// This is critical: RunCheck emits check.completed and baseline.updated
	// with the same timestamp, and replay must apply them in emit order.
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
	return events, nil
}

// NewEvent creates a new Event with a UUID, the given timestamp, and marshaled data payload.
func NewEvent(eventType EventType, data any, timestamp time.Time) (Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event data: %w", err)
	}
	return Event{
		ID:        uuid.NewString(),
		Type:      eventType,
		Timestamp: timestamp,
		Data:      raw,
	}, nil
}
