package platform_test

import (
	"context"
	"testing"

	"github.com/hironow/amadeus/internal/platform"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestRecordCheck_IncreasesCounter(t *testing.T) {
	// given
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	origMeter := platform.Meter
	platform.Meter = mp.Meter("test")
	defer func() { platform.Meter = origMeter }()
	ctx := context.Background()

	// when
	platform.RecordCheck(ctx, "clean")
	platform.RecordCheck(ctx, "drift")
	platform.RecordCheck(ctx, "clean")

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
