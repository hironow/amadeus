package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PruneCandidate represents an archive file eligible for pruning.
type PruneCandidate struct {
	Path    string
	ModTime time.Time
}

// FindPruneCandidates returns .md files in archiveDir older than maxAge.
// Returns (nil, nil) if the directory does not exist.
func FindPruneCandidates(archiveDir string, maxAge time.Duration) ([]PruneCandidate, error) {
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	cutoff := time.Now().Add(-maxAge)
	candidates := []PruneCandidate{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", e.Name(), err)
		}
		if info.ModTime().Before(cutoff) {
			candidates = append(candidates, PruneCandidate{
				Path:    filepath.Join(archiveDir, e.Name()),
				ModTime: info.ModTime(),
			})
		}
	}
	return candidates, nil
}

// PruneFiles deletes the given files and returns the count of successfully deleted files.
func PruneFiles(candidates []PruneCandidate) (int, error) {
	count := 0
	for _, c := range candidates {
		if err := os.Remove(c.Path); err != nil {
			return count, fmt.Errorf("remove %s: %w", c.Path, err)
		}
		count++
	}
	return count, nil
}
