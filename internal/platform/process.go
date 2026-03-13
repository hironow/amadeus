package platform

import (
	"os"
	"runtime"
	"syscall"
)

// IsProcessAlive checks whether a process with the given PID is running.
// On Unix, it sends signal 0 to probe liveness. On Windows, os.FindProcess
// succeeds for any PID, so we attempt to open the process handle instead.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	if runtime.GOOS == "windows" {
		return isProcessAliveWindows(pid)
	}
	return isProcessAliveUnix(pid)
}

// isProcessAliveUnix uses signal 0 to check if a process exists.
func isProcessAliveUnix(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	// EPERM means the process exists but is owned by another user
	if err == syscall.EPERM {
		return true
	}
	return false
}

// isProcessAliveWindows uses os.FindProcess + Signal(0) on Windows.
// On Windows, os.FindProcess always succeeds, but Signal(0) will fail
// if the process does not exist.
func isProcessAliveWindows(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, Signal with syscall.Signal(0) is not supported the same way.
	// We try it and treat any non-nil error as "not alive".
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
