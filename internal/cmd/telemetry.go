package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/hironow/amadeus/internal/platform"
)

func mergeResource(base *resource.Resource, extra *resource.Resource) *resource.Resource {
	merged, err := resource.Merge(base, extra)
	if err != nil {
		fmt.Fprintf(os.Stderr, "otel resource merge warning: %v (continuing with base resource)\n", err)
		return base
	}
	return merged
}

// initTracer sets up the OpenTelemetry TracerProvider and updates
// platform.Tracer. Called from PersistentPreRunE; shutdown is registered
// via cobra.OnFinalize.
func initTracer(serviceName, ver string) func(context.Context) error {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		np := noop.NewTracerProvider()
		otel.SetTracerProvider(np) // nosemgrep: adr0003-otel-set-tracer-provider-outside-init — initTracer is the composition-root initializer (unexported equivalent of InitTracer) [permanent]
		platform.Tracer = np.Tracer(serviceName)
		return func(context.Context) error { return nil }
	}

	exp, err := otlptracehttp.New(context.Background())
	if err != nil {
		np := noop.NewTracerProvider()
		otel.SetTracerProvider(np) // nosemgrep: adr0003-otel-set-tracer-provider-outside-init — initTracer is the composition-root initializer (unexported equivalent of InitTracer) [permanent]
		platform.Tracer = np.Tracer(serviceName)
		return func(context.Context) error { return nil }
	}

	res := mergeResource(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(ver),
		),
	)

	if entity := os.Getenv("WANDB_ENTITY"); entity != "" {
		res = mergeResource(res, resource.NewWithAttributes(
			semconv.SchemaURL,
			attribute.String("wandb.entity", platform.SanitizeUTF8(entity)),
			attribute.String("wandb.project", platform.SanitizeUTF8(os.Getenv("WANDB_PROJECT"))),
		))
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	}

	for _, ep := range parseExtraEndpoints(os.Getenv("OTEL_EXPORTER_OTLP_EXTRA_ENDPOINTS")) {
		extra, extraErr := otlptracehttp.New(context.Background(),
			otlptracehttp.WithEndpoint(ep),
			otlptracehttp.WithInsecure(),
		)
		if extraErr == nil {
			opts = append(opts, sdktrace.WithBatcher(extra))
		}
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp) // nosemgrep: adr0003-otel-set-tracer-provider-outside-init — initTracer is the composition-root initializer (unexported equivalent of InitTracer) [permanent]
	platform.Tracer = tp.Tracer(serviceName)

	return func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}
}

// initMeter sets up the OTel meter provider. When OTEL_EXPORTER_OTLP_METRICS_ENDPOINT
// is set, it creates an OTLP HTTP metric exporter; otherwise it stays noop.
func initMeter(serviceName, ver string) func(context.Context) error {
	if os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT") == "" {
		mp := metricnoop.NewMeterProvider()
		platform.Meter = mp.Meter(serviceName)
		return func(context.Context) error { return nil }
	}

	exp, err := otlpmetrichttp.New(context.Background(),
		otlpmetrichttp.WithEndpoint(os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		mp := metricnoop.NewMeterProvider()
		platform.Meter = mp.Meter(serviceName)
		return func(context.Context) error { return nil }
	}

	res := mergeResource(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(ver),
		),
	)

	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(exp)),
		metric.WithResource(res),
	)
	platform.Meter = mp.Meter(serviceName)

	return func(ctx context.Context) error {
		return mp.Shutdown(ctx)
	}
}

// rootSpan holds the top-level span for the CLI invocation.
// It is set by startRootSpan and closed by endRootSpan (called from
// cobra.OnFinalize, which runs even on error — unlike PersistentPostRunE).
var rootSpan trace.Span

// startRootSpan creates the top-level span for an amadeus subcommand and
// returns a new context carrying it. The span is stored in the package-level
// rootSpan variable so endRootSpan can close it without a context argument.
func startRootSpan(ctx context.Context, command string) context.Context {
	ctx, rootSpan = platform.Tracer.Start(ctx, "amadeus."+command,
		trace.WithAttributes(
			attribute.String("amadeus.command", platform.SanitizeUTF8(command)),
		),
	)
	return ctx
}

// endRootSpan ends the package-level rootSpan (created by startRootSpan).
// Safe to call when rootSpan is nil (e.g., before startRootSpan runs).
func endRootSpan() {
	if rootSpan != nil {
		rootSpan.End()
	}
}

// parseExtraEndpoints splits a comma-separated list of OTLP endpoints.
func parseExtraEndpoints(envVal string) []string {
	if envVal == "" {
		return nil
	}
	var endpoints []string
	for _, ep := range strings.Split(envVal, ",") {
		ep = strings.TrimSpace(ep)
		if ep != "" {
			endpoints = append(endpoints, ep)
		}
	}
	return endpoints
}
