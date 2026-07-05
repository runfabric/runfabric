package telemetry

import (
	"context"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

// envCarrier exposes the TRACEPARENT/TRACESTATE environment variables as a
// TextMapCarrier (env var name = uppercased header name — the convention used
// by otel-cli and CI systems) so one-shot CLI invocations can join the
// distributed trace of the process that spawned them.
type envCarrier struct{}

func (envCarrier) Get(key string) string { return os.Getenv(strings.ToUpper(key)) }
func (envCarrier) Set(string, string)    {}
func (envCarrier) Keys() []string        { return []string{"traceparent", "tracestate"} }

// ContextFromEnv returns ctx enriched with the W3C trace context carried in
// the TRACEPARENT / TRACESTATE environment variables, if present and valid.
// Extraction is explicit (not the global propagator) so it works
// deterministically even when Init has not run.
func ContextFromEnv(ctx context.Context) context.Context {
	return propagation.TraceContext{}.Extract(ctx, envCarrier{})
}

// StartRootSpan starts a span that joins any trace context already on ctx
// (e.g. from ContextFromEnv); with no inbound context it starts a new root.
// The returned end func records err on the span and ends it — call it before
// Shutdown so the span flushes.
func StartRootSpan(ctx context.Context, tracerName, spanName string) (context.Context, func(error)) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, spanName)
	return ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
}
