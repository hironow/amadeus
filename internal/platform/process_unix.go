//go:build !windows

package platform

import (
	"errors"
	"os"
	"syscall"
)

// IsProcessAlive checks whether a process with the given PID is still running.
// On Unix, it sends signal 0 to the process. Returns false for invalid PIDs.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	// EPERM means the process exists but is owned by another user
	if errors.Is(err, syscall.EPERM) {
		return true
	}
	return false
}
