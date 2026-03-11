package platform

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// RecordCheck increments the amadeus.check.total OTel counter.
func RecordCheck(ctx context.Context, status string) {
	c, _ := Meter.Int64Counter("amadeus.check.total",
		metric.WithDescription("Total check completions"),
	)
	c.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", SanitizeUTF8(status)),
		),
	)
}
