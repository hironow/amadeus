package usecase

import (
	"context"
	"fmt"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// RunCheck orchestrates the amadeus check pipeline.
// This is the reference implementation of COMMAND → Aggregate → EVENT:
//  1. Validate the ExecuteCheckCommand
//  2. Create CheckAggregate, wrap in EventEmitter + StateManager
//  3. Wire policy engine (WHEN [EVENT] THEN [handler])
//  4. Delegate I/O pipeline to session via port.Orchestrator
func RunCheck(ctx context.Context, cmd domain.ExecuteCheckCommand, opts domain.CheckOptions,
	pipeline port.Orchestrator, cfg domain.Config, logger domain.Logger,
	notifier port.Notifier, metrics port.PolicyMetrics) error {
	// COMMAND validation
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
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
	registerCheckPolicies(engine, logger, notifier, metrics)

	// Create EventEmitter + StateManager wrapping the aggregate
	emitter := NewCheckEventEmitter(agg, pipeline.EventStore(), pipeline.EventApplier(), engine, logger)
	state := NewCheckStateProvider(agg)

	// Delegate to session I/O pipeline via Orchestrator interface
	// Session restores aggregate state from persisted projection internally
	return pipeline.RunCheck(ctx, opts, emitter, state)
}
