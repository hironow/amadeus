package usecase

import (
	"context"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// ExportRegisterCheckPolicies exposes registerCheckPolicies for external tests.
var ExportRegisterCheckPolicies = func(engine *PolicyEngine, logger domain.Logger, notifier port.Notifier, metrics port.PolicyMetrics) {
	registerCheckPolicies(engine, logger, notifier, metrics)
}

// ExportPolicyHandler is a type alias for external tests.
type ExportPolicyHandler = func(ctx context.Context, ev domain.Event) error
