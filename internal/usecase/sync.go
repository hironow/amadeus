package usecase

import (
	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// PrintSync orchestrates the sync status output.
// The RunSyncCommand is already valid by construction (parse-don't-validate).
func PrintSync(cmd domain.RunSyncCommand, pipeline port.Orchestrator) error {
	_ = cmd // command validated by construction; no fields accessed here
	return pipeline.PrintSync()
}
