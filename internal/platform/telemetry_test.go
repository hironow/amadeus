package platform

// white-box-reason: platform internals: tests unexported tracer setup and span recording

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func setupTestTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	Tracer = tp.Tracer("amadeus-test")
	t.Cleanup(func() {
		tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
		Tracer = prev.Tracer("amadeus")
	})
	return exp
}

func TestSetupTestTracer_RecordsSpans(t *testing.T) {
	exp := setupTestTracer(t)

	_, span := Tracer.Start(context.Background(), "recording-span") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, immediately ended [permanent]
	span.End()

	spans := exp.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least 1 span in InMemoryExporter")
	}
	if spans[0].Name != "recording-span" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "recording-span")
	}
}
