package amadeus

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

func TestInitTracer_NoopWhenEndpointUnset(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown := InitTracer("test-svc", "0.0.1")
	defer shutdown(context.Background())

	_, span := Tracer.Start(context.Background(), "test-span")
	defer span.End()

	if span.IsRecording() {
		t.Error("span should NOT be recording when endpoint is unset (noop provider)")
	}
}

func TestSetupTestTracer_RecordsSpans(t *testing.T) {
	exp := setupTestTracer(t)

	_, span := Tracer.Start(context.Background(), "recording-span")
	span.End()

	spans := exp.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least 1 span in InMemoryExporter")
	}
	if spans[0].Name != "recording-span" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "recording-span")
	}
}
