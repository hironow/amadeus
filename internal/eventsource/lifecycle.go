package eventsource

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExpiredFile represents an event file eligible for pruning.
type ExpiredFile struct {
	Path    string
	ModTime time.Time
}

// FindExpiredEventFiles returns .jsonl files in eventsDir older than maxAge.
// Returns (nil, nil) if the directory does not exist.
func FindExpiredEventFiles(eventsDir string, maxAge time.Duration) ([]ExpiredFile, error) {
	entries, err := os.ReadDir(eventsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	cutoff := time.Now().Add(-maxAge)
	var expired []ExpiredFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", e.Name(), err)
		}
		if info.ModTime().Before(cutoff) {
			expired = append(expired, ExpiredFile{
				Path:    filepath.Join(eventsDir, e.Name()),
				ModTime: info.ModTime(),
			})
		}
	}
	return expired, nil
}

// PruneEventFiles deletes the given expired event files.
// Returns the count of successfully deleted files.
func PruneEventFiles(files []ExpiredFile) (int, error) {
	count := 0
	for _, f := range files {
		if err := os.Remove(f.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return count, fmt.Errorf("remove %s: %w", f.Path, err)
		}
		count++
	}
	return count, nil
}
