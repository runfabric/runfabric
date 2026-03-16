package telemetry

import (
	"context"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var tp *sdktrace.TracerProvider

// Init initializes OpenTelemetry tracing when an OTLP endpoint is set
// (OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_EXPORTER_OTLP_TRACES_ENDPOINT, or
// OTEL_TRACES_ENABLED=1 with default localhost:4318). Otherwise a no-op provider is used.
// Call Shutdown before process exit to flush spans.
func Init(ctx context.Context) error {
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "runfabric"
	}
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	}
	// Default OTLP HTTP endpoint when OTEL_TRACES_ENABLED=1 but no endpoint set
	tracesEnabled := os.Getenv("OTEL_TRACES_ENABLED") == "1" || os.Getenv("OTEL_TRACES_ENABLED") == "true"
	if endpoint == "" && tracesEnabled {
		endpoint = "localhost:4318"
	}
	if endpoint == "" {
		tp = sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample()))
		otel.SetTracerProvider(tp)
		return nil
	}
	// OTLP HTTP exporter expects host:port (no scheme)
	if strings.HasPrefix(endpoint, "https://") {
		endpoint = strings.TrimPrefix(endpoint, "https://")
	} else if strings.HasPrefix(endpoint, "http://") {
		endpoint = strings.TrimPrefix(endpoint, "http://")
	}
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return err
	}
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return err
	}
	tp = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	return nil
}

// Tracer returns a Tracer for the given name (e.g. "runfabric/daemon", "runfabric/deploy").
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// Shutdown flushes and shuts down the TracerProvider. Call from main before exit.
func Shutdown(ctx context.Context) error {
	if tp != nil {
		return tp.Shutdown(ctx)
	}
	return nil
}
