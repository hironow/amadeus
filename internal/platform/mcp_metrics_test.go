package platform_test

import (
	"context"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/platform"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestRecordMCPInvocation_IncrementsCounterAndHistogram(t *testing.T) {
	// given
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	origMeter := platform.Meter
	platform.Meter = mp.Meter("test")
	defer func() { platform.Meter = origMeter }()
	ctx := context.Background()

	// when
	platform.RecordMCPInvocation(ctx, "amadeus.ping", "ok", 5*time.Millisecond)
	platform.RecordMCPInvocation(ctx, "amadeus.next_review", "deprecated", 12*time.Millisecond)
	platform.RecordMCPInvocation(ctx, "amadeus.ping", "ok", 3*time.Millisecond)

	// then
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatal(err)
	}
	if total := sumCounter(t, rm, "mcp.tool.invocations"); total != 3 {
		t.Errorf("invocations total = %d, want 3", total)
	}
	if count := mcpHistogramCount(t, rm, "mcp.tool.duration"); count != 3 {
		t.Errorf("duration count = %d, want 3", count)
	}
}

func TestRecordMCPInvocation_AttributesIncludeToolNameAndStatus(t *testing.T) {
	// given
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	origMeter := platform.Meter
	platform.Meter = mp.Meter("test")
	defer func() { platform.Meter = origMeter }()
	ctx := context.Background()

	// when
	platform.RecordMCPInvocation(ctx, "amadeus.post_comment", "error", 2*time.Millisecond)

	// then
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatal(err)
	}
	foundTool, foundStatus := false, false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "mcp.tool.invocations" {
				continue
			}
			sum := m.Data.(metricdata.Sum[int64])
			for _, dp := range sum.DataPoints {
				for _, attr := range dp.Attributes.ToSlice() {
					if string(attr.Key) == "tool.name" && attr.Value.AsString() == "amadeus.post_comment" {
						foundTool = true
					}
					if string(attr.Key) == "result.status" && attr.Value.AsString() == "error" {
						foundStatus = true
					}
				}
			}
		}
	}
	if !foundTool {
		t.Error("expected tool.name=amadeus.post_comment attribute on metric data point")
	}
	if !foundStatus {
		t.Error("expected result.status=error attribute on metric data point")
	}
}

// mcpHistogramCount sums data point counts across the histogram metric
// of the given name. Local helper namespaced to avoid collision.
func mcpHistogramCount(t *testing.T, rm metricdata.ResourceMetrics, name string) uint64 {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			hist := m.Data.(metricdata.Histogram[float64])
			var total uint64
			for _, dp := range hist.DataPoints {
				total += dp.Count
			}
			return total
		}
	}
	t.Fatalf("histogram %q not found", name)
	return 0
}
