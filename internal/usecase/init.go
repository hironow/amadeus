package usecase

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// RunInit validates the InitCommand and delegates state directory creation
// to the InitRunner port.
func RunInit(cmd domain.InitCommand, runner port.InitRunner) error {
	if errs := cmd.Validate(); len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return fmt.Errorf("invalid init command: %s", strings.Join(msgs, "; "))
	}
	stateDir := filepath.Join(cmd.RepoRoot, domain.StateDir)
	return runner.InitGateDir(stateDir)
}
