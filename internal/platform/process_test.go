package platform

// white-box-reason: platform internals: tests isProcessAlive helper for cross-platform PID liveness check

import (
	"os"
	"runtime"
	"testing"
)

func TestIsProcessAlive_CurrentProcess(t *testing.T) {
	t.Parallel()

	// given: the current process PID (always alive)
	pid := os.Getpid()

	// when
	alive := IsProcessAlive(pid)

	// then
	if !alive {
		t.Errorf("expected current process (PID %d) to be alive", pid)
	}
}

func TestIsProcessAlive_NonExistentProcess(t *testing.T) {
	t.Parallel()

	// given: a PID that almost certainly doesn't exist
	pid := 99999999

	// when
	alive := IsProcessAlive(pid)

	// then
	if alive {
		t.Errorf("expected PID %d to not be alive", pid)
	}
}

func TestIsProcessAlive_InvalidPID(t *testing.T) {
	t.Parallel()

	// given: invalid PID values
	for _, pid := range []int{0, -1} {
		// when
		alive := IsProcessAlive(pid)

		// then
		if alive {
			t.Errorf("expected PID %d to not be alive", pid)
		}
	}
}

func TestIsProcessAlive_RuntimeGOOS(t *testing.T) {
	// Verify we handle the current OS (documents cross-platform support)
	t.Logf("running on GOOS=%s", runtime.GOOS)

	// given: current process
	pid := os.Getpid()

	// when
	alive := IsProcessAlive(pid)

	// then: must work on current platform
	if !alive {
		t.Errorf("isProcessAlive must work on %s", runtime.GOOS)
	}
}
