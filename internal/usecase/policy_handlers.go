package usecase

import (
	"context"

	"github.com/hironow/amadeus/internal/domain"
)

// registerCheckPolicies registers POLICY handlers for check events.
// See ADR S0014 (POLICY pattern) and S0018 (Event Storming alignment).
func registerCheckPolicies(engine *PolicyEngine, logger *domain.Logger) {
	engine.Register(domain.EventCheckCompleted, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: check completed (type=%s)", event.Type)
		return nil
	})

	engine.Register(domain.EventConvergenceDetected, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: convergence detected (type=%s)", event.Type)
		return nil
	})

	engine.Register(domain.EventInboxConsumed, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: inbox consumed (type=%s)", event.Type)
		return nil
	})

	engine.Register(domain.EventDMailGenerated, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: dmail generated (type=%s)", event.Type)
		return nil
	})
}
