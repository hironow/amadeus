package cmd

import (
	"context"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/hironow/amadeus"
)

// initTracer sets up the OpenTelemetry TracerProvider and updates
// amadeus.Tracer. Called from PersistentPreRunE; shutdown is registered
// via cobra.OnFinalize.
func initTracer(serviceName, ver string) func(context.Context) error {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		np := noop.NewTracerProvider()
		otel.SetTracerProvider(np)
		amadeus.Tracer = np.Tracer(serviceName)
		return func(context.Context) error { return nil }
	}

	exp, err := otlptracehttp.New(context.Background())
	if err != nil {
		np := noop.NewTracerProvider()
		otel.SetTracerProvider(np)
		amadeus.Tracer = np.Tracer(serviceName)
		return func(context.Context) error { return nil }
	}

	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(ver),
		),
	)

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
	otel.SetTracerProvider(tp)
	amadeus.Tracer = tp.Tracer(serviceName)

	return func(ctx context.Context) error {
		return tp.Shutdown(ctx)
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
	ctx, rootSpan = amadeus.Tracer.Start(ctx, "amadeus."+command,
		trace.WithAttributes(
			attribute.String("amadeus.command", command),
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
