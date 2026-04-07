package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// registerCheckPolicies registers POLICY handlers for check events.
// See ADR S0014 (POLICY pattern) and S0018 (Event Storming alignment).
func registerCheckPolicies(engine *PolicyEngine, logger domain.Logger, notifier port.Notifier, metrics port.PolicyMetrics, dispatcher port.ImprovementTaskDispatcher) {
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

	// POLICY: convergence.detected → notify + metrics.
	// Convergence detection indicates a recurring pattern — notify user.
	engine.Register(domain.EventConvergenceDetected, func(ctx context.Context, event domain.Event) error {
		logger.Info("policy: convergence detected (type=%s)", event.Type)
		notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := notifier.Notify(notifyCtx, "Amadeus", "Convergence detected"); err != nil {
			logger.Debug("policy: notify error: %v", err)
		}
		metrics.RecordPolicyEvent(ctx, "convergence.detected", "handled")
		return nil
	})

	// POLICY CONTRACT: observation-only — debug log + metrics.
	// Inbox consumption is an intermediate processing step.
	engine.Register(domain.EventInboxConsumed, func(ctx context.Context, event domain.Event) error {
		logger.Debug("policy: inbox consumed (type=%s)", event.Type)
		metrics.RecordPolicyEvent(ctx, "inbox.consumed", "handled")
		return nil
	})

	// POLICY: dmail.generated → dispatch improvement task on escalation only.
	// Improvement tasks are created only when routing_mode=escalate (escalation
	// or recurrence threshold exceeded). retry/reroute D-Mails are not dispatched
	// as improvement tasks — they follow the normal delivery path.
	engine.Register(domain.EventDMailGenerated, func(ctx context.Context, event domain.Event) error {
		logger.Debug("policy: dmail generated (type=%s)", event.Type)
		metrics.RecordPolicyEvent(ctx, "dmail.generated", "handled")

		var data domain.DMailGeneratedData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			logger.Debug("policy: dmail generated parse error: %v", err)
			return nil
		}
		meta := domain.CorrectionMetadataFromMap(data.DMail.Metadata)
		if meta.RoutingMode != domain.RoutingModeEscalate || meta.CorrelationID == "" {
			return nil
		}
		task := domain.NewImprovementTask(
			data.DMail.Name,
			meta.TargetAgent,
			meta.CorrectiveAction,
			meta.FailureType,
			meta.Severity,
			30*time.Minute,
		)
		if dispatchErr := dispatcher.Dispatch(ctx, task, meta.CorrelationID); dispatchErr != nil {
			logger.Debug("policy: improvement task dispatch: %v", dispatchErr)
		} else {
			metrics.RecordPolicyEvent(ctx, "improvement.task.dispatched", "handled")
		}
		return nil
	})
}
