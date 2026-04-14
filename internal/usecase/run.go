package usecase

import (
	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// WirePRPipeline connects the PR convergence pipeline to the orchestrator.
// Called by cmd when prReader is available.
func WirePRPipeline(pipeline port.Orchestrator, prReader port.GitHubPRReader, stateReader port.StateReader, emitter port.CheckEventEmitter, logger domain.Logger) {
	pipeline.SetPRPipeline(NewPRPipelineRunner(prReader, stateReader, emitter, logger))
}
