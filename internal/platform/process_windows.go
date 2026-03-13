//go:build windows

package platform

import (
	"os/exec"
	"strconv"
)

// IsProcessAlive checks whether a process with the given PID is still running.
// On Windows, os.FindProcess always succeeds, so we use tasklist to verify.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// On Windows, FindProcess always succeeds regardless of whether the
	// process exists. Use tasklist with a PID filter instead.
	cmd := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	// tasklist output contains the PID when the process is found
	return len(out) > 0 && containsPID(out, pid)
}

// containsPID checks whether tasklist output contains the given PID.
func containsPID(output []byte, pid int) bool {
	pidStr := strconv.Itoa(pid)
	s := string(output)
	// tasklist /NH /FI "PID eq X" returns "INFO: No tasks..." when not found
	if len(s) > 4 && s[:4] == "INFO" {
		return false
	}
	// Check if the output contains the PID string
	for i := 0; i <= len(s)-len(pidStr); i++ {
		if s[i:i+len(pidStr)] == pidStr {
			return true
		}
	}
	return false
}
