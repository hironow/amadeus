package eventsource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// snapshotEnvelope is the on-disk JSON format for a snapshot file.
type snapshotEnvelope struct {
	SeqNr         uint64          `json:"seq_nr"`
	AggregateType string          `json:"aggregate_type"`
	Timestamp     time.Time       `json:"timestamp"`
	State         json.RawMessage `json:"state"`
}

// FileSnapshotStore implements SnapshotStore using atomic file writes.
// Each aggregate type gets a single file: {dir}/{aggregateType}.json
type FileSnapshotStore struct {
	dir string
}

// NewFileSnapshotStore creates a FileSnapshotStore rooted at the given directory.
func NewFileSnapshotStore(dir string) *FileSnapshotStore {
	return &FileSnapshotStore{dir: dir}
}

// Save persists a snapshot using atomic write (temp file + rename).
func (s *FileSnapshotStore) Save(_ context.Context, aggregateType string, seqNr uint64, state []byte) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("snapshot store: create dir: %w", err)
	}

	envelope := snapshotEnvelope{
		SeqNr:         seqNr,
		AggregateType: aggregateType,
		Timestamp:     time.Now(),
		State:         json.RawMessage(state),
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("snapshot store: marshal: %w", err)
	}

	target := filepath.Join(s.dir, aggregateType+".json")
	tmp, err := os.CreateTemp(s.dir, "snapshot-*.tmp")
	if err != nil {
		return fmt.Errorf("snapshot store: create temp: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("snapshot store: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("snapshot store: sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("snapshot store: close temp: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("snapshot store: rename: %w", err)
	}
	return nil
}

// Load returns the latest snapshot for the given aggregateType.
// Returns (0, nil, nil) if no snapshot exists.
func (s *FileSnapshotStore) Load(_ context.Context, aggregateType string) (uint64, []byte, error) {
	path := filepath.Join(s.dir, aggregateType+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil, nil
		}
		return 0, nil, fmt.Errorf("snapshot store: read: %w", err)
	}

	var envelope snapshotEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return 0, nil, fmt.Errorf("snapshot store: unmarshal: %w", err)
	}
	return envelope.SeqNr, []byte(envelope.State), nil
}
