package platform

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// OTelPolicyMetrics implements port.PolicyMetrics using OTel counters.
type OTelPolicyMetrics struct{}

func (*OTelPolicyMetrics) RecordPolicyEvent(ctx context.Context, eventType, status string) {
	c, _ := Meter.Int64Counter("amadeus.harness.event.total",
		metric.WithDescription("Policy handler execution count"),
	)
	c.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("event_type", SanitizeUTF8(eventType)),
			attribute.String("status", SanitizeUTF8(status)),
		),
	)
}
