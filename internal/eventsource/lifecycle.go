package eventsource

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EventFileSizeThreshold is the file size in bytes above which an event file
// is considered oversized and eligible for truncation (10 MiB).
const EventFileSizeThreshold = 10 * 1024 * 1024

// EventFileTruncateKeepLines is the number of most-recent lines to retain
// when truncating an oversized event file.
const EventFileTruncateKeepLines = 1000

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

// ListOversizedEventFiles returns the names of .jsonl event files under stateDir
// whose size exceeds EventFileSizeThreshold. Today's file is always excluded
// because it may still be actively written.
// Returns (nil, nil) if the events directory does not exist.
func ListOversizedEventFiles(stateDir string) ([]string, error) {
	dir := EventsDir(stateDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	today := time.Now().Format("2006-01-02") + ".jsonl"
	var oversized []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		if e.Name() == today {
			continue
		}
		info, infoErr := e.Info()
		if infoErr != nil {
			continue
		}
		if info.Size() > EventFileSizeThreshold {
			oversized = append(oversized, e.Name())
		}
	}
	return oversized, nil
}

// TruncateEventFile rewrites the named .jsonl file inside stateDir's events
// directory, keeping only the last keepLines lines. The write is atomic:
// content is written to a temporary file and then renamed over the original.
func TruncateEventFile(stateDir, name string, keepLines int) error {
	dir := EventsDir(stateDir)
	path := filepath.Join(dir, name)

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open event file %s: %w", name, err)
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		f.Close()
		return fmt.Errorf("scan event file %s: %w", name, scanErr)
	}
	f.Close()

	if len(lines) <= keepLines {
		return nil // already within limit
	}
	lines = lines[len(lines)-keepLines:]

	// Write atomically: tmp file then rename
	tmpPath := path + ".tmp"
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create tmp file for %s: %w", name, err)
	}
	w := bufio.NewWriter(out)
	for _, line := range lines {
		if _, writeErr := w.WriteString(line + "\n"); writeErr != nil {
			out.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("write tmp file for %s: %w", name, writeErr)
		}
	}
	if flushErr := w.Flush(); flushErr != nil {
		out.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("flush tmp file for %s: %w", name, flushErr)
	}
	if syncErr := out.Sync(); syncErr != nil {
		out.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync tmp file for %s: %w", name, syncErr)
	}
	out.Close()

	if renameErr := os.Rename(tmpPath, path); renameErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename tmp file for %s: %w", name, renameErr)
	}
	return nil
}
