package telemetry

import (
	"context"
	"os"
	"strconv"
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
		tp = sdktrace.NewTracerProvider(sdktrace.WithSampler(samplerFromEnv()))
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
		sdktrace.WithSampler(samplerFromEnv()),
	)
	otel.SetTracerProvider(tp)
	return nil
}

// samplerFromEnv returns a Sampler from OTEL_TRACES_SAMPLER and OTEL_TRACES_SAMPLER_ARG.
// Supported: "always_on", "always_off", "traceidratio" (arg = ratio 0..1), "parentbased_always_on", "parentbased_traceidratio" (arg = ratio).
// Default when exporter is set: AlwaysSample(); when no exporter: NeverSample().
func samplerFromEnv() sdktrace.Sampler {
	s := strings.TrimSpace(strings.ToLower(os.Getenv("OTEL_TRACES_SAMPLER")))
	argStr := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG"))
	switch s {
	case "always_off", "never":
		return sdktrace.NeverSample()
	case "traceidratio":
		if ratio := parseRatio(argStr); ratio >= 0 {
			return sdktrace.TraceIDRatioBased(ratio)
		}
		return sdktrace.AlwaysSample()
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "parentbased_traceidratio":
		if ratio := parseRatio(argStr); ratio >= 0 {
			return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
		}
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "always_on", "always", "":
		return sdktrace.AlwaysSample()
	default:
		return sdktrace.AlwaysSample()
	}
}

func parseRatio(s string) float64 {
	if s == "" {
		return -1
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 || f > 1 {
		return -1
	}
	return f
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
