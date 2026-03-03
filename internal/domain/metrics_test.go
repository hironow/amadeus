package domain_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	amadeus "github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/domain"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func makeCheckEvent(dmails []string) domain.Event {
	data, _ := json.Marshal(domain.CheckCompletedData{
		Result: amadeus.CheckResult{DMails: dmails},
	})
	return domain.Event{ID: "test", Type: domain.EventCheckCompleted, Timestamp: time.Now(), Data: data}
}

func TestSuccessRate_AllClean(t *testing.T) {
	events := []domain.Event{
		makeCheckEvent(nil),
		makeCheckEvent(nil),
	}

	rate := domain.SuccessRate(events)

	if rate != 1.0 {
		t.Errorf("SuccessRate = %f, want 1.0", rate)
	}
}

func TestSuccessRate_AllDrift(t *testing.T) {
	events := []domain.Event{
		makeCheckEvent([]string{"feedback-001"}),
		makeCheckEvent([]string{"feedback-002"}),
	}

	rate := domain.SuccessRate(events)

	if rate != 0.0 {
		t.Errorf("SuccessRate = %f, want 0.0", rate)
	}
}

func TestSuccessRate_Mixed(t *testing.T) {
	events := []domain.Event{
		makeCheckEvent(nil),
		makeCheckEvent([]string{"feedback-001"}),
		makeCheckEvent(nil),
	}

	rate := domain.SuccessRate(events)

	if rate < 0.66 || rate > 0.67 {
		t.Errorf("SuccessRate = %f, want ~0.666", rate)
	}
}

func TestSuccessRate_NoEvents(t *testing.T) {
	rate := domain.SuccessRate(nil)

	if rate != 0.0 {
		t.Errorf("SuccessRate = %f, want 0.0", rate)
	}
}

func TestSuccessRate_IgnoresOtherEvents(t *testing.T) {
	events := []domain.Event{
		{ID: "1", Type: domain.EventBaselineUpdated, Timestamp: time.Now()},
		makeCheckEvent(nil),
		{ID: "3", Type: domain.EventDMailGenerated, Timestamp: time.Now()},
	}

	rate := domain.SuccessRate(events)

	if rate != 1.0 {
		t.Errorf("SuccessRate = %f, want 1.0", rate)
	}
}

func TestRecordCheck_IncreasesCounter(t *testing.T) {
	// given
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	origMeter := amadeus.Meter
	amadeus.Meter = mp.Meter("test")
	defer func() { amadeus.Meter = origMeter }()
	ctx := context.Background()

	// when
	domain.RecordCheck(ctx, "clean")
	domain.RecordCheck(ctx, "drift")
	domain.RecordCheck(ctx, "clean")

	// then
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatal(err)
	}
	total := sumCounter(t, rm, "amadeus.check.total")
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
}

func sumCounter(t *testing.T, rm metricdata.ResourceMetrics, name string) int64 {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				sum := m.Data.(metricdata.Sum[int64])
				var total int64
				for _, dp := range sum.DataPoints {
					total += dp.Value
				}
				return total
			}
		}
	}
	t.Fatalf("metric %q not found", name)
	return 0
}

func TestFormatSuccessRate_WithEvents(t *testing.T) {
	// given
	rate := 0.857142
	success := 6
	total := 7

	// when
	msg := domain.FormatSuccessRate(rate, success, total)

	// then
	if msg != "85.7% (6/7)" {
		t.Errorf("FormatSuccessRate = %q, want %q", msg, "85.7% (6/7)")
	}
}

func TestFormatSuccessRate_NoEvents(t *testing.T) {
	// given
	rate := 0.0
	success := 0
	total := 0

	// when
	msg := domain.FormatSuccessRate(rate, success, total)

	// then
	if msg != "no events" {
		t.Errorf("FormatSuccessRate = %q, want %q", msg, "no events")
	}
}
