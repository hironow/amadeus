package usecase

import (
	"context"
	"fmt"

	amadeus "github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/session"
)

// RunCheck orchestrates the amadeus check pipeline.
// This is the reference implementation of COMMAND → Aggregate → EVENT:
//  1. Validate the ExecuteCheckCommand
//  2. Create and restore CheckAggregate from persisted state
//  3. Inject aggregate into session for domain decisions
//  4. Delegate I/O pipeline to session
func RunCheck(ctx context.Context, cmd amadeus.ExecuteCheckCommand, opts amadeus.CheckOptions, a *session.Amadeus) error {
	// COMMAND validation
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
	}

	// Create aggregate with config
	agg := amadeus.NewCheckAggregate(a.Config)

	// Inject aggregate into session (session uses it for domain decisions)
	a.Aggregate = agg

	// Wire policy engine (WHEN [EVENT] THEN [handler])
	// Handlers will be registered here as policies are implemented.
	engine := NewPolicyEngine(a.Logger)
	a.Dispatcher = engine

	// Delegate to session I/O pipeline
	// Session restores aggregate state from persisted projection internally
	return a.RunCheck(ctx, opts)
}
