package cmd

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/hironow/amadeus/internal/platform"
)

func TestInitTracer_NoopWhenEndpointUnset(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown := initTracer("test-svc", "0.0.1")
	defer shutdown(context.Background())

	_, span := platform.Tracer.Start(context.Background(), "test-span")
	defer span.End()

	if span.IsRecording() {
		t.Error("span should NOT be recording when endpoint is unset (noop provider)")
	}
}

func TestMultiExporter_BothReceive(t *testing.T) {
	exp1 := tracetest.NewInMemoryExporter()
	exp2 := tracetest.NewInMemoryExporter()

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp1),
		sdktrace.WithSyncer(exp2),
	)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	oldTracer := platform.Tracer
	platform.Tracer = tp.Tracer("amadeus-test")
	t.Cleanup(func() {
		tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
		platform.Tracer = oldTracer
	})

	_, span := platform.Tracer.Start(context.Background(), "multi-span") // nosemgrep: adr0003-otel-span-without-defer-end -- test span, immediately ended [permanent]
	span.End()

	if len(exp1.GetSpans()) == 0 {
		t.Error("exporter 1 received no spans")
	}
	if len(exp2.GetSpans()) == 0 {
		t.Error("exporter 2 received no spans")
	}
}

func TestParseExtraEndpoints_CommaSeparated(t *testing.T) {
	eps := parseExtraEndpoints("localhost:4318,weave.local:4318")
	if len(eps) != 2 {
		t.Fatalf("got %d endpoints, want 2", len(eps))
	}
	if eps[0] != "localhost:4318" {
		t.Errorf("eps[0] = %q, want %q", eps[0], "localhost:4318")
	}
	if eps[1] != "weave.local:4318" {
		t.Errorf("eps[1] = %q, want %q", eps[1], "weave.local:4318")
	}
}

func TestParseExtraEndpoints_Empty(t *testing.T) {
	eps := parseExtraEndpoints("")
	if len(eps) != 0 {
		t.Errorf("got %d endpoints, want 0", len(eps))
	}
}

func TestParseExtraEndpoints_Whitespace(t *testing.T) {
	eps := parseExtraEndpoints("  localhost:4318 , weave:4318  ")
	if len(eps) != 2 {
		t.Fatalf("got %d endpoints, want 2", len(eps))
	}
	if eps[0] != "localhost:4318" {
		t.Errorf("eps[0] = %q, want %q", eps[0], "localhost:4318")
	}
}

func TestStartRootSpan_CreatesNamedSpan(t *testing.T) {
	// given
	exp := setupTestTracer(t)

	// when
	_ = startRootSpan(context.Background(), "check")
	endRootSpan()

	// then
	spans := exp.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least 1 span")
	}
	if spans[0].Name != "amadeus.check" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "amadeus.check")
	}
	var found bool
	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == "amadeus.command" && attr.Value.AsString() == "check" {
			found = true
		}
	}
	if !found {
		t.Error("expected amadeus.command=check attribute on root span")
	}
}

func TestEndRootSpan_NilSafe(t *testing.T) {
	// given — rootSpan is nil (no startRootSpan called)
	rootSpan = nil

	// when / then — must not panic
	endRootSpan()
}
