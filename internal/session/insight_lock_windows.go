//go:build windows

package session

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// lock acquires an exclusive lock on insights.lock using LockFileEx.
// The returned function releases the lock.
func (w *InsightWriter) lock() (func(), error) {
	lockPath := filepath.Join(w.runDir, "insights.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", lockPath, err)
	}

	ol := new(windows.Overlapped)
	err = windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK,
		0, 1, 0, ol,
	)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("LockFileEx %s: %w", lockPath, err)
	}

	return func() {
		windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol) //nolint:errcheck
		f.Close()
	}, nil
}
