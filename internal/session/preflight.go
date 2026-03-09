package session

import (
	"fmt"

	"github.com/hironow/amadeus/internal/platform"
)

// PreflightCheck verifies that required binaries are available in PATH.
// Uses platform.LookPathShell to handle env-prefixed commands like
// "CLAUDE_CONFIG_DIR=~/.claude claude".
func PreflightCheck(binaries ...string) error {
	for _, bin := range binaries {
		if _, err := platform.LookPathShell(bin); err != nil {
			return fmt.Errorf("preflight: %s not found in PATH", bin)
		}
	}
	return nil
}
