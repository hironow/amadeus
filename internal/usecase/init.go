package usecase

import (
	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// RunInit delegates state directory creation to the InitRunner port.
// The InitCommand is already valid by construction (parse-don't-validate).
// Passes through lang override for config merge (3-stage: defaults → existing → CLI).
func RunInit(cmd domain.InitCommand, runner port.InitRunner) ([]string, error) {
	var opts []port.InitOption
	if cmd.Lang() != "" {
		opts = append(opts, port.WithLang(cmd.Lang()))
	}
	return runner.InitProject(cmd.RepoRoot().String(), opts...)
}
