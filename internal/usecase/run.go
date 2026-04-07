package usecase

import (
	"context"
	"fmt"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// Run orchestrates the amadeus daemon loop.
// This follows the same COMMAND -> Aggregate -> EVENT pattern as RunCheck:
//  1. Create CheckAggregate, wrap in EventEmitter + StateProvider
//  2. Wire policy engine (WHEN [EVENT] THEN [handler])
//  3. Wire PRPipelineRunner if prReader is non-nil
//  4. Delegate I/O loop to session via port.Orchestrator
//
// The ExecuteRunCommand is already valid by construction (parse-don't-validate).
func Run(ctx context.Context, _ domain.ExecuteRunCommand, opts domain.RunOptions,
	pipeline port.Orchestrator, cfg domain.Config, logger domain.Logger,
	notifier port.Notifier, metrics port.PolicyMetrics,
	prReader port.GitHubPRReader, stateReader port.StateReader,
	dispatcher port.ImprovementTaskDispatcher,
) error {
	// Validate event store availability
	store := pipeline.EventStore()
	if store == nil {
		return fmt.Errorf("event store is required")
	}

	// Create aggregate with config
	agg := domain.NewCheckAggregate(cfg)

	// Wire policy engine (WHEN [EVENT] THEN [handler])
	engine := NewPolicyEngine(logger)
	if notifier == nil {
		notifier = &port.NopNotifier{}
	}
	if metrics == nil {
		metrics = &port.NopPolicyMetrics{}
	}
	registerCheckPolicies(engine, logger, notifier, metrics, dispatcher)

	// Create EventEmitter + StateProvider wrapping the aggregate
	emitter := NewCheckEventEmitter(agg, store, pipeline.EventApplier(), engine, pipeline.SeqAllocator(), logger)
	state := NewCheckStateProvider(agg)

	// Wire PRPipelineRunner if prReader is available
	if prReader != nil {
		pipeline.SetPRPipeline(NewPRPipelineRunner(prReader, stateReader, emitter, logger))
	}

	// Delegate to session I/O loop via Orchestrator interface
	return pipeline.Run(ctx, opts, emitter, state)
}
