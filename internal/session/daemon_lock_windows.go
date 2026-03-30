//go:build windows

package session

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// TryLockDaemon acquires an exclusive lock on daemon.lock using LockFileEx.
func TryLockDaemon(stateDir string) (func(), error) {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}

	lockPath := filepath.Join(stateDir, "daemon.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	ol := new(windows.Overlapped)
	err = windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, ol,
	)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("daemon already running (lock held on %s)", lockPath)
	}

	unlock := func() {
		windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol) //nolint:errcheck
		f.Close()
	}
	return unlock, nil
}
