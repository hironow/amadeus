package session

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// SpanEventStore wraps a port.EventStore with OTEL span instrumentation.
// Each operation is wrapped in a span with event count and stats attributes.
type SpanEventStore struct {
	inner port.EventStore
}

// NewSpanEventStore creates a span-instrumented EventStore wrapper.
func NewSpanEventStore(inner port.EventStore) port.EventStore {
	return &SpanEventStore{inner: inner}
}

func (s *SpanEventStore) Append(ctx context.Context, events ...domain.Event) (domain.AppendResult, error) {
	ctx, span := platform.Tracer.Start(ctx, "eventsource.append")
	defer span.End()

	span.SetAttributes(attribute.Int("event.count.in", len(events)))
	result, err := s.inner.Append(ctx, events...)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "eventsource.append"))
		return result, err
	}
	span.SetAttributes(attribute.Int("event.append.bytes", result.BytesWritten))
	return result, nil
}

func (s *SpanEventStore) LoadAll(ctx context.Context) ([]domain.Event, domain.LoadResult, error) {
	ctx, span := platform.Tracer.Start(ctx, "eventsource.load_all")
	defer span.End()

	events, result, err := s.inner.LoadAll(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "eventsource.load_all"))
		return events, result, err
	}
	span.SetAttributes(attribute.Int("event.count.out", len(events)))
	span.SetAttributes(
		attribute.Int("event.file.count", result.FileCount),
		attribute.Int("event.corrupt_line.count", result.CorruptLineCount),
	)
	return events, result, nil
}

func (s *SpanEventStore) LoadSince(ctx context.Context, after time.Time) ([]domain.Event, domain.LoadResult, error) {
	ctx, span := platform.Tracer.Start(ctx, "eventsource.load_since")
	defer span.End()

	events, result, err := s.inner.LoadSince(ctx, after)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "eventsource.load_since"))
		return events, result, err
	}
	span.SetAttributes(attribute.Int("event.count.out", len(events)))
	span.SetAttributes(
		attribute.Int("event.file.count", result.FileCount),
		attribute.Int("event.corrupt_line.count", result.CorruptLineCount),
	)
	return events, result, nil
}

func (s *SpanEventStore) LoadAfterSeqNr(ctx context.Context, afterSeqNr uint64) ([]domain.Event, domain.LoadResult, error) {
	ctx, span := platform.Tracer.Start(ctx, "eventsource.load_after_seq_nr")
	defer span.End()

	span.SetAttributes(attribute.Int64("event.after_seq_nr", int64(afterSeqNr)))
	events, result, err := s.inner.LoadAfterSeqNr(ctx, afterSeqNr)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "eventsource.load_after_seq_nr"))
		return events, result, err
	}
	span.SetAttributes(attribute.Int("event.count.out", len(events)))
	return events, result, nil
}

func (s *SpanEventStore) LatestSeqNr(ctx context.Context) (uint64, error) {
	ctx, span := platform.Tracer.Start(ctx, "eventsource.latest_seq_nr")
	defer span.End()

	seqNr, err := s.inner.LatestSeqNr(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "eventsource.latest_seq_nr"))
		return 0, err
	}
	span.SetAttributes(attribute.Int64("event.latest_seq_nr", int64(seqNr)))
	return seqNr, nil
}
