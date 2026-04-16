package platform

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// RecordCheck increments the amadeus.check.total OTel counter.
func RecordCheck(ctx context.Context, status string) {
	c, err := Meter.Int64Counter("amadeus.check.total",
		metric.WithDescription("Total check completions"),
	)
	if err != nil {
		return
	}
	c.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", SanitizeUTF8(status)),
		),
	)
}
