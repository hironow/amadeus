package session

import (
	"fmt"
	"os/exec"
)

// PreflightCheck verifies that required binaries are available in PATH.
// Unlike a full doctor check, this only uses exec.LookPath (no version execution).
func PreflightCheck(binaries ...string) error {
	for _, bin := range binaries {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("preflight: %s not found in PATH", bin)
		}
	}
	return nil
}
