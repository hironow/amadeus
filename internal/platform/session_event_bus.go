package platform

import (
	"context"
	"sync"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

const publishTimeout = 100 * time.Millisecond

// BusSubscriber receives session stream events from an InProcessSessionBus.
type BusSubscriber struct {
	ch   chan domain.SessionStreamEvent
	done chan struct{}
	once sync.Once
}

// C returns a read-only channel of events.
func (s *BusSubscriber) C() <-chan domain.SessionStreamEvent {
	return s.ch
}

// Close unsubscribes and drains the channel.
func (s *BusSubscriber) Close() {
	s.once.Do(func() {
		close(s.done)
	})
}

// InProcessSessionBus is a fan-out pub/sub hub for session stream events.
// Publish is best-effort with a short timeout per subscriber.
type InProcessSessionBus struct {
	mu          sync.RWMutex
	subscribers []*BusSubscriber
	closed      bool
}

// NewInProcessSessionBus creates a new bus.
func NewInProcessSessionBus() *InProcessSessionBus {
	return &InProcessSessionBus{}
}

// Publish sends an event to all subscribers. Non-blocking: if a subscriber
// channel is full after publishTimeout, the event is dropped for that subscriber.
// This is an observation stream — drops do not affect domain event persistence.
func (b *InProcessSessionBus) Publish(ctx context.Context, event domain.SessionStreamEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return
	}
	for _, sub := range b.subscribers {
		select {
		case <-sub.done:
			continue
		default:
		}
		select {
		case sub.ch <- event:
		case <-time.After(publishTimeout):
			// Drop: subscriber too slow. This is observation-only;
			// domain event store is unaffected.
		case <-ctx.Done():
			return
		case <-sub.done:
		}
	}
}

// Subscribe creates a new subscriber with a buffered channel.
func (b *InProcessSessionBus) Subscribe(bufSize int) *BusSubscriber {
	if bufSize < 1 {
		bufSize = 64
	}
	sub := &BusSubscriber{
		ch:   make(chan domain.SessionStreamEvent, bufSize),
		done: make(chan struct{}),
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		close(sub.done)
		return sub
	}
	b.subscribers = append(b.subscribers, sub)
	return sub
}

// Close closes all subscriber channels and prevents further publishes.
func (b *InProcessSessionBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for _, sub := range b.subscribers {
		sub.Close()
		close(sub.ch)
	}
	b.subscribers = nil
}
