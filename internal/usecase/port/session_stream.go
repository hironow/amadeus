package port

import (
	"context"

	"github.com/hironow/amadeus/internal/domain"
)

// SessionStreamPublisher publishes session stream events to subscribers.
type SessionStreamPublisher interface { // nosemgrep: structure.multiple-exported-interfaces-go -- session stream port triple (SessionStreamPublisher/SessionStreamSubscriber/SessionStreamBus) is a cohesive pub/sub port family; splitting would fragment the bus contract [permanent]
	Publish(ctx context.Context, event domain.SessionStreamEvent)
}

// SessionStreamSubscriber receives session stream events.
type SessionStreamSubscriber interface { // nosemgrep: structure.multiple-exported-interfaces-go -- session stream port family cohesive set; see SessionStreamPublisher [permanent]
	C() <-chan domain.SessionStreamEvent
	Close()
}

// SessionStreamBus manages pub/sub for session stream events.
type SessionStreamBus interface {
	SessionStreamPublisher
	Subscribe(bufSize int) SessionStreamSubscriber
	Close()
}
