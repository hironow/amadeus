package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// InsightWriter provides atomic read/write access to insight ledger files.
// It uses flock-based locking for concurrent safety and temp-file-rename
// for atomicity. Title-based dedup ensures idempotent appends.
type InsightWriter struct {
	insightsDir string
	runDir      string
}

// NewInsightWriter creates an InsightWriter for the given directories.
func NewInsightWriter(insightsDir, runDir string) *InsightWriter {
	return &InsightWriter{insightsDir: insightsDir, runDir: runDir}
}

// Append adds a new InsightEntry to the named file, creating it if needed.
// Uses flock + atomic rename for concurrent safety.
// Idempotent: skips if an entry with the same title already exists.
func (w *InsightWriter) Append(filename, kind, tool string, entry domain.InsightEntry) error {
	path := filepath.Join(w.insightsDir, filename)

	unlock, err := w.lock()
	if err != nil {
		return fmt.Errorf("acquire insight lock: %w", err)
	}
	defer unlock()

	// Read existing file; create new only on ENOENT, propagate other errors.
	file, err := w.readFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read existing insight file %s: %w", path, err)
		}
		file = &domain.InsightFile{
			SchemaVersion: domain.InsightSchemaVersion,
			Kind:          kind,
			Tool:          tool,
		}
	}

	// Idempotency: skip if entry with same title already exists.
	for _, existing := range file.Entries {
		if existing.Title == entry.Title {
			return nil
		}
	}

	file.Entries = append(file.Entries, entry)
	file.UpdatedAt = time.Now()

	data, err := file.Marshal()
	if err != nil {
		return fmt.Errorf("marshal insight file: %w", err)
	}

	// Atomic write: temp file + rename.
	// NOTE: On Windows, os.Rename fails if destination exists.
	// For cross-platform safety, consider using a rename-with-replace strategy.
	// Current design targets Linux/macOS where rename is atomic.
	tmpPath := filepath.Join(w.insightsDir, "."+filename+".tmp")
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp insight file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // best-effort cleanup
		return fmt.Errorf("rename insight file: %w", err)
	}

	return nil
}

// Read parses an insight file. Safe without locking due to atomic rename writes.
func (w *InsightWriter) Read(filename string) (*domain.InsightFile, error) {
	path := filepath.Join(w.insightsDir, filename)
	return w.readFile(path)
}

func (w *InsightWriter) readFile(path string) (*domain.InsightFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return domain.UnmarshalInsightFile(data)
}
