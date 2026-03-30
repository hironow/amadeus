package platform_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
)

func testEvent(typ domain.StreamEventType) domain.SessionStreamEvent {
	return domain.NewSessionStreamEvent("test-tool", domain.ProviderClaudeCode, typ, nil)
}

func TestInProcessSessionBus_PublishSubscribe(t *testing.T) {
	t.Parallel()
	bus := platform.NewInProcessSessionBus()
	defer bus.Close()

	sub := bus.Subscribe(16)
	defer sub.Close()

	ev := testEvent(domain.StreamToolUseStart)
	bus.Publish(context.Background(), ev)

	select {
	case got := <-sub.C():
		if got.Type != domain.StreamToolUseStart {
			t.Errorf("Type = %q, want %q", got.Type, domain.StreamToolUseStart)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestInProcessSessionBus_FanOut(t *testing.T) {
	t.Parallel()
	bus := platform.NewInProcessSessionBus()
	defer bus.Close()

	sub1 := bus.Subscribe(16)
	defer sub1.Close()
	sub2 := bus.Subscribe(16)
	defer sub2.Close()

	ev := testEvent(domain.StreamAssistantText)
	bus.Publish(context.Background(), ev)

	for i, sub := range []*platform.BusSubscriber{sub1, sub2} {
		select {
		case got := <-sub.C():
			if got.Type != domain.StreamAssistantText {
				t.Errorf("sub%d: Type = %q, want %q", i, got.Type, domain.StreamAssistantText)
			}
		case <-time.After(time.Second):
			t.Fatalf("sub%d: timeout", i)
		}
	}
}

func TestInProcessSessionBus_SlowSubscriberDoesNotBlock(t *testing.T) {
	t.Parallel()
	bus := platform.NewInProcessSessionBus()
	defer bus.Close()

	// Buffer of 1 — second publish should not block indefinitely.
	sub := bus.Subscribe(1)
	defer sub.Close()

	ev := testEvent(domain.StreamToolUseStart)
	bus.Publish(context.Background(), ev)
	bus.Publish(context.Background(), ev) // This may be dropped.

	// Should complete without deadlock (publish timeout is 100ms).
	select {
	case <-sub.C():
	case <-time.After(time.Second):
		t.Fatal("first event should be received")
	}
}

func TestInProcessSessionBus_CloseStopsPublish(t *testing.T) {
	t.Parallel()
	bus := platform.NewInProcessSessionBus()
	sub := bus.Subscribe(16)

	bus.Close()

	// Publish after close should not panic.
	bus.Publish(context.Background(), testEvent(domain.StreamSessionStart))

	// Subscriber channel should be closed.
	_, open := <-sub.C()
	if open {
		t.Error("subscriber channel should be closed after bus.Close()")
	}
}

func TestInProcessSessionBus_ConcurrentPublish(t *testing.T) {
	t.Parallel()
	bus := platform.NewInProcessSessionBus()
	defer bus.Close()

	sub := bus.Subscribe(256)
	defer sub.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				bus.Publish(context.Background(), testEvent(domain.StreamToolUseStart))
			}
		}()
	}
	wg.Wait()

	// Drain and count.
	count := 0
	for {
		select {
		case <-sub.C():
			count++
		default:
			goto done
		}
	}
done:
	if count < 50 { // At least half should arrive given 256 buffer.
		t.Errorf("expected >=50 events, got %d", count)
	}
}

func TestInProcessSessionBus_SubscriberClose(t *testing.T) {
	t.Parallel()
	bus := platform.NewInProcessSessionBus()
	defer bus.Close()

	sub := bus.Subscribe(16)
	sub.Close()

	// Publish after subscriber close should not panic or block.
	bus.Publish(context.Background(), testEvent(domain.StreamSessionEnd))
}
