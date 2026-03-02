package eventsource

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ListExpiredEventFiles returns .jsonl file names in the events directory
// under stateDir that are older than the given number of days.
// Returns (nil, nil) if the events directory does not exist.
func ListExpiredEventFiles(stateDir string, days int) ([]string, error) {
	if days < 0 {
		return nil, fmt.Errorf("days must be non-negative, got %d", days)
	}
	dir := EventsDir(stateDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	var expired []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, infoErr := e.Info()
		if infoErr != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			expired = append(expired, e.Name())
		}
	}
	return expired, nil
}

// PruneEventFiles deletes the named .jsonl files from the events directory
// under stateDir. Returns the list of successfully deleted file names.
func PruneEventFiles(stateDir string, files []string) ([]string, error) {
	dir := EventsDir(stateDir)
	var deleted []string
	for _, name := range files {
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return deleted, err
		}
		deleted = append(deleted, name)
	}
	return deleted, nil
}
