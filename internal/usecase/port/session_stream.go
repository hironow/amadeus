package port

import (
	"context"

	"github.com/hironow/amadeus/internal/domain"
)

// SessionStreamPublisher publishes session stream events to subscribers.
type SessionStreamPublisher interface {
	Publish(ctx context.Context, event domain.SessionStreamEvent)
}

// SessionStreamSubscriber receives session stream events.
type SessionStreamSubscriber interface {
	C() <-chan domain.SessionStreamEvent
	Close()
}

// SessionStreamBus manages pub/sub for session stream events.
type SessionStreamBus interface {
	SessionStreamPublisher
	Subscribe(bufSize int) SessionStreamSubscriber
	Close()
}
