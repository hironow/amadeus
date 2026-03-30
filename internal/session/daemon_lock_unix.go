//go:build !windows

package session

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// TryLockDaemon acquires an exclusive advisory lock on daemon.lock in the
// given directory. If another process holds the lock, it returns an error
// immediately (LOCK_NB). The returned function releases the lock.
// The OS automatically releases the lock if the process crashes.
func TryLockDaemon(stateDir string) (func(), error) {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}

	lockPath := filepath.Join(stateDir, "daemon.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("daemon already running (lock held on %s)", lockPath)
	}

	unlock := func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck
		f.Close()
	}
	return unlock, nil
}
