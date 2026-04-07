package usecase

// white-box-reason: bridge constructor: exposes unexported symbols for external test packages

import (
	"context"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// ExportRegisterCheckPolicies exposes registerCheckPolicies for external tests.
var ExportRegisterCheckPolicies = func(engine *PolicyEngine, logger domain.Logger, notifier port.Notifier, metrics port.PolicyMetrics, dispatcher port.ImprovementTaskDispatcher) {
	registerCheckPolicies(engine, logger, notifier, metrics, dispatcher)
}

// ExportPolicyHandler is a type alias for external tests.
type ExportPolicyHandler = func(ctx context.Context, ev domain.Event) error
