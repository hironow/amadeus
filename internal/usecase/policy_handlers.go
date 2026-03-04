package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/port"
)

// registerCheckPolicies registers POLICY handlers for check events.
// See ADR S0014 (POLICY pattern) and S0018 (Event Storming alignment).
func registerCheckPolicies(engine *PolicyEngine, logger domain.Logger, notifier port.Notifier, metrics port.PolicyMetrics) {
	engine.Register(domain.EventCheckCompleted, func(ctx context.Context, event domain.Event) error {
		var data domain.CheckCompletedData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			logger.Debug("policy: check completed parse error: %v", err)
			return nil
		}
		logger.Info("policy: check completed (divergence=%.2f, commit=%s)", data.Result.Divergence, data.Result.Commit)
		notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := notifier.Notify(notifyCtx, "Amadeus",
			fmt.Sprintf("Check completed: divergence=%.2f, commit=%s",
				data.Result.Divergence, data.Result.Commit)); err != nil {
			logger.Debug("policy: notify error: %v", err)
		}
		metrics.RecordPolicyEvent(ctx, "check.completed", "handled")
		return nil
	})

	engine.Register(domain.EventConvergenceDetected, func(ctx context.Context, event domain.Event) error {
		logger.Debug("policy: convergence detected (type=%s)", event.Type)
		metrics.RecordPolicyEvent(ctx, "convergence.detected", "handled")
		return nil
	})

	engine.Register(domain.EventInboxConsumed, func(ctx context.Context, event domain.Event) error {
		logger.Debug("policy: inbox consumed (type=%s)", event.Type)
		metrics.RecordPolicyEvent(ctx, "inbox.consumed", "handled")
		return nil
	})

	engine.Register(domain.EventDMailGenerated, func(ctx context.Context, event domain.Event) error {
		logger.Debug("policy: dmail generated (type=%s)", event.Type)
		metrics.RecordPolicyEvent(ctx, "dmail.generated", "handled")
		return nil
	})
}
