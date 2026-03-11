package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// TestWeave_LiveTraceDelivery sends a minimal span to the real W&B Weave
// OTLP endpoint and verifies the exporter completes without error.
//
// Prerequisites:
//   - WANDB_API_KEY must be set (skipped otherwise)
//   - WANDB_ENTITY defaults to "hironow"
//   - WANDB_PROJECT defaults to "four-tools-test"
//
// This test is opt-in: it only runs when WANDB_API_KEY is present.
func TestWeave_LiveTraceDelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	apiKey := os.Getenv("WANDB_API_KEY")
	if apiKey == "" {
		t.Skip("skipping: WANDB_API_KEY not set")
	}

	entity := envOrDefault("WANDB_ENTITY", "hironow")
	project := envOrDefault("WANDB_PROJECT", "four-tools-test")

	// Configure OTLP exporter for real Weave endpoint.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "https://trace.wandb.ai/otel")
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "wandb-api-key="+apiKey)
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES",
		"wandb.entity="+entity+",wandb.project="+project)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		t.Fatalf("create OTLP HTTP exporter: %v", err)
	}

	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("amadeus"),
			semconv.ServiceVersion("0.0.0-live-test"),
			attribute.String("wandb.entity", entity),   // nosemgrep: otel-attribute-string-unsanitized -- env config value, always ASCII [permanent]
			attribute.String("wandb.project", project), // nosemgrep: otel-attribute-string-unsanitized -- env config value, always ASCII [permanent]
		),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
		sdktrace.WithResource(res),
	)

	tracer := tp.Tracer("amadeus")
	_, span := tracer.Start(ctx, "live-weave-verification") // nosemgrep: adr0003-otel-span-without-defer-end — test span, immediately ended [permanent]
	span.SetAttributes(
		attribute.String("test.tool", "amadeus"),
		attribute.String("test.type", "live-verification"),
	)
	span.End()

	if err := tp.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown tracer provider (span delivery failed): %v", err)
	}

	t.Logf("span delivered to W&B Weave: entity=%s project=%s service=amadeus", entity, project)
	t.Logf("verify at: https://wandb.ai/%s/%s/weave", entity, project)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
