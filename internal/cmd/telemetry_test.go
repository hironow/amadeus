package cmd

import (
	"context"
	"testing"

	"github.com/hironow/amadeus"
)

func TestInitTracer_NoopWhenEndpointUnset(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown := InitTracer("test-svc", "0.0.1")
	defer shutdown(context.Background())

	_, span := amadeus.Tracer.Start(context.Background(), "test-span")
	defer span.End()

	if span.IsRecording() {
		t.Error("span should NOT be recording when endpoint is unset (noop provider)")
	}
}
