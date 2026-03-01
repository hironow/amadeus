package usecase

import (
	"context"

	amadeus "github.com/hironow/amadeus"
)

// registerCheckPolicies registers POLICY handlers for check events.
// See ADR S0014 (POLICY pattern) and S0018 (Event Storming alignment).
func registerCheckPolicies(engine *PolicyEngine, logger *amadeus.Logger) {
	engine.Register(amadeus.EventCheckCompleted, func(_ context.Context, event amadeus.Event) error {
		logger.Debug("policy: check completed (type=%s)", event.Type)
		return nil
	})

	engine.Register(amadeus.EventConvergenceDetected, func(_ context.Context, event amadeus.Event) error {
		logger.Debug("policy: convergence detected (type=%s)", event.Type)
		return nil
	})

	engine.Register(amadeus.EventInboxConsumed, func(_ context.Context, event amadeus.Event) error {
		logger.Debug("policy: inbox consumed (type=%s)", event.Type)
		return nil
	})

	engine.Register(amadeus.EventDMailGenerated, func(_ context.Context, event amadeus.Event) error {
		logger.Debug("policy: dmail generated (type=%s)", event.Type)
		return nil
	})
}
