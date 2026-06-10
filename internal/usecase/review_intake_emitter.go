package usecase

import (
	"context"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// reviewIntakeEmitter appends review-intake events to the gate event
// store (domain constructors produce; the store persists — dominator
// ADR 0005 pattern).
type reviewIntakeEmitter struct {
	ctx    context.Context
	store  port.EventStore
	logger domain.Logger
}

// NewReviewIntakeEmitter wires the reviewer write path for the MCP
// composition root.
func NewReviewIntakeEmitter(ctx context.Context, store port.EventStore, logger domain.Logger) port.ReviewIntakeEmitter {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	return &reviewIntakeEmitter{ctx: ctx, store: store, logger: logger}
}

func (e *reviewIntakeEmitter) EmitPRSnapshotIngested(prs []domain.PRSnapshotEntry, now time.Time) error {
	ev, err := domain.NewPRSnapshotIngestedEvent(prs, now)
	if err != nil {
		return err
	}
	_, err = e.store.Append(e.ctx, ev)
	return err
}

func (e *reviewIntakeEmitter) EmitReviewPosted(prNumber string, now time.Time) error {
	ev, err := domain.NewReviewPostedEvent(prNumber, now)
	if err != nil {
		return err
	}
	_, err = e.store.Append(e.ctx, ev)
	return err
}
