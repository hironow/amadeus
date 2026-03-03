package usecase

import (
	"context"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/port"
)

// Compile-time check: PolicyEngine implements port.EventDispatcher.
var _ port.EventDispatcher = (*PolicyEngine)(nil)

// PolicyHandler processes a domain event as part of a policy reaction.
// WHEN [EVENT] THEN [handler logic].
type PolicyHandler func(ctx context.Context, event domain.Event) error

// PolicyEngine dispatches domain events to registered policy handlers.
// This connects the POLICY registry (domain.Policies) to executable handlers.
type PolicyEngine struct {
	handlers map[domain.EventType][]PolicyHandler
	logger   domain.Logger
}

// NewPolicyEngine creates a PolicyEngine. Pass nil logger for silent operation.
func NewPolicyEngine(logger domain.Logger) *PolicyEngine {
	return &PolicyEngine{
		handlers: make(map[domain.EventType][]PolicyHandler),
		logger:   logger,
	}
}

// Register adds a handler for the given event type.
// Multiple handlers can be registered for the same event type.
func (e *PolicyEngine) Register(trigger domain.EventType, handler PolicyHandler) {
	e.handlers[trigger] = append(e.handlers[trigger], handler)
}

// Dispatch sends an event to all handlers registered for its type.
// Best-effort: handler errors are logged but never block event processing.
func (e *PolicyEngine) Dispatch(ctx context.Context, event domain.Event) error {
	handlers, ok := e.handlers[event.Type]
	if !ok {
		return nil
	}
	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			if e.logger != nil {
				e.logger.Debug("policy dispatch %s: %v", event.Type, err)
			}
		}
	}
	return nil
}
