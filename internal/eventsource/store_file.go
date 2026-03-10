package eventsource

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// FileEventStore implements EventStore using daily JSONL files in a directory.
// Each file is named YYYY-MM-DD.jsonl and contains one JSON event per line.
type FileEventStore struct {
	dir    string
	logger domain.Logger
}

// NewFileEventStore creates a FileEventStore rooted at the given directory.
func NewFileEventStore(dir string, logger domain.Logger) *FileEventStore {
	return &FileEventStore{dir: dir, logger: logger}
}

// Append persists events as JSONL lines to the daily file based on each event's timestamp.
// All events are validated before any writes occur; if any event is invalid, the entire batch is rejected.
func (s *FileEventStore) Append(events ...domain.Event) (domain.AppendResult, error) {
	for _, ev := range events {
		if err := domain.ValidateEvent(ev); err != nil {
			return domain.AppendResult{}, fmt.Errorf("validate event %s: %w", ev.ID, err)
		}
	}

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return domain.AppendResult{}, fmt.Errorf("create event store dir: %w", err)
	}

	// Group events by date for file routing
	byDate := make(map[string][]domain.Event)
	for _, ev := range events {
		date := ev.Timestamp.Format("2006-01-02")
		byDate[date] = append(byDate[date], ev)
	}

	var totalBytes int
	for date, evs := range byDate {
		filename := filepath.Join(s.dir, date+".jsonl")
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return domain.AppendResult{}, fmt.Errorf("open event file %s: %w", date, err)
		}
		for _, ev := range evs {
			line, err := json.Marshal(ev)
			if err != nil {
				f.Close()
				return domain.AppendResult{}, fmt.Errorf("marshal event %s: %w", ev.ID, err)
			}
			data := append(line, '\n')
			totalBytes += len(data)
			if _, err := f.Write(data); err != nil {
				f.Close()
				return domain.AppendResult{}, fmt.Errorf("write event %s: %w", ev.ID, err)
			}
		}
		if err := f.Sync(); err != nil {
			f.Close()
			return domain.AppendResult{}, fmt.Errorf("fsync event file %s: %w", date, err)
		}
		if err := f.Close(); err != nil {
			return domain.AppendResult{}, fmt.Errorf("close event file %s: %w", date, err)
		}
	}
	return domain.AppendResult{BytesWritten: totalBytes}, nil
}

// LoadAll reads all JSONL files in lexicographic order and returns events chronologically.
func (s *FileEventStore) LoadAll() ([]domain.Event, domain.LoadResult, error) {
	return s.loadEvents(time.Time{})
}

// LoadSince returns events with timestamps strictly after the given time.
func (s *FileEventStore) LoadSince(after time.Time) ([]domain.Event, domain.LoadResult, error) {
	return s.loadEvents(after)
}

func (s *FileEventStore) loadEvents(after time.Time) ([]domain.Event, domain.LoadResult, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, domain.LoadResult{}, nil
		}
		return nil, domain.LoadResult{}, fmt.Errorf("read event store dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	var corruptCount int
	var events []domain.Event
	for _, name := range files {
		path := filepath.Join(s.dir, name)
		f, err := os.Open(path)
		if err != nil {
			return nil, domain.LoadResult{}, fmt.Errorf("open %s: %w", name, err)
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // allow up to 10MiB per line
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var ev domain.Event
			if jsonErr := json.Unmarshal(line, &ev); jsonErr != nil {
				s.logger.Warn("corrupt event line in %s, skipping: %v", name, jsonErr)
				corruptCount++
				continue
			}
			if !after.IsZero() && !ev.Timestamp.After(after) {
				continue
			}
			events = append(events, ev)
		}
		if err := scanner.Err(); err != nil {
			f.Close()
			return nil, domain.LoadResult{}, fmt.Errorf("scan %s: %w", name, err)
		}
		f.Close()
	}

	// Stable sort preserves insertion order for events with equal timestamps.
	// This is critical: RunCheck emits check.completed and baseline.updated
	// with the same timestamp, and replay must apply them in emit order.
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
	return events, domain.LoadResult{FileCount: len(files), CorruptLineCount: corruptCount}, nil
}
