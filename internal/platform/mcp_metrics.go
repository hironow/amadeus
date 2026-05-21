package platform

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// RecordMCPInvocation increments mcp.tool.invocations counter and
// records mcp.tool.duration histogram for a `tools/call` invocation
// on the amadeus MCP server.
//
// Phase 3 (refs/issues/0027) cost monitoring (a): every MCP tool call
// is counted with (tool.name, result.status) attrs so credit-pool 0
// consumption can be verified post 2026-06-15 via OTel + Anthropic
// dashboard cross-check. paintress mcp_metrics.go is the reference
// impl; this file is a symmetric copy adapted for amadeus.
//
// status values: "ok" (= JSON-RPC result returned)、 "error" (= JSON-RPC
// error returned)、 "deprecated" (= stub returned with stub:true flag).
// duration is measured from request decode to response write.
func RecordMCPInvocation(ctx context.Context, toolName, status string, duration time.Duration) {
	attrs := metric.WithAttributes(
		attribute.String("tool.name", SanitizeUTF8(toolName)),
		attribute.String("result.status", SanitizeUTF8(status)),
	)

	counter, err := Meter.Int64Counter("mcp.tool.invocations",
		metric.WithDescription("Total MCP tools/call invocations on amadeus MCP server"),
	)
	if err == nil {
		counter.Add(ctx, 1, attrs)
	}

	histogram, err := Meter.Float64Histogram("mcp.tool.duration",
		metric.WithDescription("Duration (seconds) of MCP tools/call invocations on amadeus MCP server"),
		metric.WithUnit("s"),
	)
	if err == nil {
		histogram.Record(ctx, duration.Seconds(), attrs)
	}
}
