package usecase

import (
	"context"
	"fmt"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/port"
	"github.com/hironow/amadeus/internal/session"
)

// RunCheck orchestrates the amadeus check pipeline.
// This is the reference implementation of COMMAND → Aggregate → EVENT:
//  1. Validate the ExecuteCheckCommand
//  2. Create and restore CheckAggregate from persisted state
//  3. Inject aggregate into session for domain decisions
//  4. Delegate I/O pipeline to session
func RunCheck(ctx context.Context, cmd domain.ExecuteCheckCommand, opts domain.CheckOptions, a *session.Amadeus) error {
	// COMMAND validation
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
	}

	// Create aggregate with config
	agg := domain.NewCheckAggregate(a.Config)

	// Inject aggregate into session (session uses it for domain decisions)
	a.Aggregate = agg

	// Wire policy engine (WHEN [EVENT] THEN [handler])
	engine := NewPolicyEngine(a.Logger)
	notifier := a.Notifier
	if notifier == nil {
		notifier = &port.NopNotifier{}
	}
	registerCheckPolicies(engine, a.Logger, notifier, &port.NopPolicyMetrics{})
	a.Dispatcher = engine

	// Delegate to session I/O pipeline
	// Session restores aggregate state from persisted projection internally
	return a.RunCheck(ctx, opts)
}

// RunCheckFromParams constructs an Amadeus from AmadeusParams and runs the check pipeline.
// This is the cmd-facing entry point that eliminates session imports from cmd.
func RunCheckFromParams(ctx context.Context, cmd domain.ExecuteCheckCommand, opts domain.CheckOptions, params AmadeusParams) error {
	result, err := buildAmadeus(params)
	if err != nil {
		return fmt.Errorf("build amadeus: %w", err)
	}
	defer result.Cleanup()

	return RunCheck(ctx, cmd, opts, result.Amadeus)
}
