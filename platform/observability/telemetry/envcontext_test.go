package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestContextFromEnvJoinsTrace(t *testing.T) {
	t.Setenv("TRACEPARENT", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	t.Setenv("TRACESTATE", "vendor=value")

	sc := trace.SpanContextFromContext(ContextFromEnv(context.Background()))
	if !sc.IsValid() {
		t.Fatal("expected a valid span context from TRACEPARENT")
	}
	if got := sc.TraceID().String(); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("trace id = %s", got)
	}
	if !sc.IsRemote() {
		t.Error("expected the extracted context to be marked remote")
	}
	if got := sc.TraceState().String(); got != "vendor=value" {
		t.Errorf("tracestate = %q", got)
	}
}

func TestContextFromEnvIgnoresAbsentOrMalformed(t *testing.T) {
	for name, value := range map[string]string{"absent": "", "malformed": "not-a-traceparent"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("TRACEPARENT", value)
			if sc := trace.SpanContextFromContext(ContextFromEnv(context.Background())); sc.IsValid() {
				t.Errorf("expected no span context, got trace %s", sc.TraceID())
			}
		})
	}
}
