package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase"
)

type fakeEventStore struct {
	appended []domain.Event
	err      error
}

func (s *fakeEventStore) Append(_ context.Context, events ...domain.Event) (domain.AppendResult, error) {
	if s.err != nil {
		return domain.AppendResult{}, s.err
	}
	s.appended = append(s.appended, events...)
	return domain.AppendResult{}, nil
}

func (s *fakeEventStore) LoadAll(_ context.Context) ([]domain.Event, domain.LoadResult, error) {
	return nil, domain.LoadResult{}, nil
}

func (s *fakeEventStore) LoadSince(_ context.Context, _ time.Time) ([]domain.Event, domain.LoadResult, error) {
	return nil, domain.LoadResult{}, nil
}

func (s *fakeEventStore) LoadAfterSeqNr(_ context.Context, _ uint64) ([]domain.Event, domain.LoadResult, error) {
	return nil, domain.LoadResult{}, nil
}

func (s *fakeEventStore) LatestSeqNr(_ context.Context) (uint64, error) {
	return 0, nil
}

type fakeDispatcher struct {
	dispatched []domain.Event
}

func (d *fakeDispatcher) Dispatch(_ context.Context, event domain.Event) error {
	d.dispatched = append(d.dispatched, event)
	return nil
}

func TestCheckEventEmitter_StoresAndDispatches(t *testing.T) {
	store := &fakeEventStore{}
	dispatcher := &fakeDispatcher{}
	agg := domain.NewCheckAggregate(domain.Config{})
	agg.SetCheckID("check-1")
	emitter := usecase.NewCheckEventEmitter(context.Background(), agg, store, nil, dispatcher, nil, &domain.NopLogger{}, "check-1")

	err := emitter.EmitRunStarted(domain.RunStartedData{}, time.Now())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.appended) != 1 {
		t.Errorf("expected 1 stored event, got %d", len(store.appended))
	}
	if len(dispatcher.dispatched) != 1 {
		t.Errorf("expected 1 dispatched event, got %d", len(dispatcher.dispatched))
	}
}

func TestCheckEventEmitter_CorrelationCausationChain(t *testing.T) {
	store := &fakeEventStore{}
	agg := domain.NewCheckAggregate(domain.Config{})
	agg.SetCheckID("check-1")
	emitter := usecase.NewCheckEventEmitter(context.Background(), agg, store, nil, nil, nil, &domain.NopLogger{}, "check-1")

	// Emit two events to verify causation chain
	if err := emitter.EmitRunStarted(domain.RunStartedData{}, time.Now()); err != nil {
		t.Fatalf("emit 1: %v", err)
	}
	if err := emitter.EmitRunStopped(domain.RunStoppedData{Reason: domain.RunStoppedReasonSignal}, time.Now()); err != nil {
		t.Fatalf("emit 2: %v", err)
	}

	if len(store.appended) != 2 {
		t.Fatalf("expected 2 events, got %d", len(store.appended))
	}

	ev1 := store.appended[0]
	ev2 := store.appended[1]

	// Both events should have same CorrelationID
	if ev1.CorrelationID != "check-1" {
		t.Errorf("ev1.CorrelationID = %q, want check-1", ev1.CorrelationID)
	}
	if ev2.CorrelationID != "check-1" {
		t.Errorf("ev2.CorrelationID = %q, want check-1", ev2.CorrelationID)
	}

	// ev2 should have ev1's ID as CausationID
	if ev2.CausationID != ev1.ID {
		t.Errorf("ev2.CausationID = %q, want ev1.ID %q", ev2.CausationID, ev1.ID)
	}

	// AggregateID should be set by aggregate
	if ev1.AggregateID != "check-1" {
		t.Errorf("ev1.AggregateID = %q, want check-1", ev1.AggregateID)
	}
}

func TestCheckEventEmitter_StoreFailurePropagates(t *testing.T) {
	store := &fakeEventStore{err: errors.New("disk full")}
	agg := domain.NewCheckAggregate(domain.Config{})
	emitter := usecase.NewCheckEventEmitter(context.Background(), agg, store, nil, nil, nil, &domain.NopLogger{}, "check-1")

	err := emitter.EmitRunStarted(domain.RunStartedData{}, time.Now())

	// amadeus propagates store errors
	if err == nil {
		t.Fatal("expected error from store failure")
	}
}
