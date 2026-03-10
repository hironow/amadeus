//go:build !windows

package session

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// lock acquires an exclusive advisory flock on insights.lock in the run directory.
// The returned function releases the lock.
func (w *InsightWriter) lock() (func(), error) {
	lockPath := filepath.Join(w.runDir, "insights.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", lockPath, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("flock %s: %w", lockPath, err)
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck
		f.Close()
	}, nil
}
