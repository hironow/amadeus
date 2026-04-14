package usecase

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// BuildCheckEmitter creates a CheckEventEmitter + CheckStateProvider for the amadeus pipeline.
// Called by cmd (composition root) before delegating to pipeline.RunCheck or pipeline.Run.
// The idPrefix distinguishes check vs run invocations (e.g. "check-" or "run-").
func BuildCheckEmitter(ctx context.Context, idPrefix string,
	pipeline port.Orchestrator, cfg domain.Config, logger domain.Logger,
	notifier port.Notifier, metrics port.PolicyMetrics,
	dispatcher port.ImprovementTaskDispatcher,
) (port.CheckEventEmitter, port.CheckStateProvider) {
	agg := domain.NewCheckAggregate(cfg)

	engine := NewPolicyEngine(logger)
	if notifier == nil {
		notifier = &port.NopNotifier{}
	}
	if metrics == nil {
		metrics = &port.NopPolicyMetrics{}
	}
	registerCheckPolicies(engine, logger, notifier, metrics, dispatcher)

	checkID := fmt.Sprintf("%s%d-%d", idPrefix, time.Now().UnixMilli(), os.Getpid())
	agg.SetCheckID(checkID)
	emitter := NewCheckEventEmitter(ctx, agg, pipeline.EventStore(), pipeline.EventApplier(), engine, pipeline.SeqAllocator(), logger, checkID)
	state := NewCheckStateProvider(agg)

	return emitter, state
}
