package usecase

import (
	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// RunInit delegates state directory creation to the InitRunner port.
// The InitCommand is already valid by construction (parse-don't-validate).
// amadeus init uses only baseDir (no team/project/lang/strictness options).
func RunInit(cmd domain.InitCommand, runner port.InitRunner) ([]string, error) {
	return runner.InitProject(cmd.RepoRoot().String())
}
