package amadeus

import (
	"context"
	"encoding/json"
	"fmt"

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
			attribute.String("status", status),
		),
	)
}

// SuccessRate calculates the clean check rate from a list of events.
// A check with zero D-Mails generated is considered a success.
// Only EventCheckCompleted events are considered.
// Returns 0.0 if there are no relevant events.
func SuccessRate(events []Event) float64 {
	var clean, total int
	for _, ev := range events {
		if ev.Type != EventCheckCompleted {
			continue
		}
		var data CheckCompletedData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			continue
		}
		total++
		if len(data.Result.DMails) == 0 {
			clean++
		}
	}
	if total == 0 {
		return 0.0
	}
	return float64(clean) / float64(total)
}

// FormatSuccessRate formats the success rate as a human-readable string.
// Returns "no events" when total is zero.
func FormatSuccessRate(rate float64, success, total int) string {
	if total == 0 {
		return "no events"
	}
	return fmt.Sprintf("%.1f%% (%d/%d)", rate*100, success, total)
}
