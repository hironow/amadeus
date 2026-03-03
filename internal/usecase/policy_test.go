package usecase

import (
	"context"
	"fmt"
	"testing"
	"time"

	amadeus "github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/domain"
)

func TestPolicyEngine_Dispatch_NoHandlers(t *testing.T) {
	// given
	engine := NewPolicyEngine(nil)
	ev, err := domain.NewEvent(domain.EventCheckCompleted, domain.CheckCompletedData{
		Result: amadeus.CheckResult{Commit: "abc123"},
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	dispatchErr := engine.Dispatch(context.Background(), ev)

	// then: no handlers registered → no error
	if dispatchErr != nil {
		t.Fatalf("expected no error, got: %v", dispatchErr)
	}
}

func TestPolicyEngine_RegisterAndFire(t *testing.T) {
	// given
	engine := NewPolicyEngine(nil)
	var fired bool
	engine.Register(domain.EventCheckCompleted, func(ctx context.Context, ev domain.Event) error {
		fired = true
		return nil
	})
	ev, err := domain.NewEvent(domain.EventCheckCompleted, domain.CheckCompletedData{
		Result: amadeus.CheckResult{Commit: "abc123"},
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	dispatchErr := engine.Dispatch(context.Background(), ev)

	// then
	if dispatchErr != nil {
		t.Fatalf("expected no error, got: %v", dispatchErr)
	}
	if !fired {
		t.Fatal("expected handler to fire")
	}
}

func TestPolicyEngine_MultipleHandlers(t *testing.T) {
	// given
	engine := NewPolicyEngine(nil)
	var count int
	for range 3 {
		engine.Register(domain.EventCheckCompleted, func(ctx context.Context, ev domain.Event) error {
			count++
			return nil
		})
	}
	ev, err := domain.NewEvent(domain.EventCheckCompleted, domain.CheckCompletedData{
		Result: amadeus.CheckResult{Commit: "abc123"},
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	dispatchErr := engine.Dispatch(context.Background(), ev)

	// then
	if dispatchErr != nil {
		t.Fatalf("expected no error, got: %v", dispatchErr)
	}
	if count != 3 {
		t.Fatalf("expected 3 handlers to fire, got %d", count)
	}
}

func TestPolicyEngine_HandlerError(t *testing.T) {
	// given
	engine := NewPolicyEngine(nil)
	engine.Register(domain.EventCheckCompleted, func(ctx context.Context, ev domain.Event) error {
		return fmt.Errorf("handler failed")
	})
	ev, err := domain.NewEvent(domain.EventCheckCompleted, domain.CheckCompletedData{
		Result: amadeus.CheckResult{Commit: "abc123"},
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	dispatchErr := engine.Dispatch(context.Background(), ev)

	// then: first handler error stops dispatch
	if dispatchErr == nil {
		t.Fatal("expected error from handler")
	}
}

func TestPolicyEngine_UnmatchedEventType(t *testing.T) {
	// given: register for check.completed only
	engine := NewPolicyEngine(nil)
	var fired bool
	engine.Register(domain.EventCheckCompleted, func(ctx context.Context, ev domain.Event) error {
		fired = true
		return nil
	})
	ev, err := domain.NewEvent(domain.EventBaselineUpdated, domain.BaselineUpdatedData{
		Commit: "abc123",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when: dispatch a different event type
	dispatchErr := engine.Dispatch(context.Background(), ev)

	// then: handler should not fire
	if dispatchErr != nil {
		t.Fatalf("expected no error, got: %v", dispatchErr)
	}
	if fired {
		t.Fatal("handler should not fire for unmatched event type")
	}
}
