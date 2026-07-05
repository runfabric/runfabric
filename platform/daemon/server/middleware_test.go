package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/runfabric/runfabric/platform/observability/telemetry"
)

func TestOtelMiddlewareJoinsInboundTrace(t *testing.T) {
	const inboundTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"

	var seenByHandler string
	h := otelMiddleware(telemetry.Tracer("test"), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenByHandler = r.Header.Get(traceIDHeader)
	}))
	req := httptest.NewRequest("POST", "/deploy", nil)
	req.Header.Set("traceparent", "00-"+inboundTraceID+"-00f067aa0ba902b7-01")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get(traceIDHeader); got != inboundTraceID {
		t.Fatalf("response %s = %q, want inbound trace id %q", traceIDHeader, got, inboundTraceID)
	}
	if seenByHandler != inboundTraceID {
		t.Fatalf("request header %s = %q, want inbound trace id %q (access log reads this)", traceIDHeader, seenByHandler, inboundTraceID)
	}
}

func TestOtelMiddlewareNoInboundTraceStillServes(t *testing.T) {
	h := otelMiddleware(telemetry.Tracer("test"), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/deploy", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestRequestIDMiddlewareGeneratesAndEchoes(t *testing.T) {
	var seen string
	h := requestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get(requestIDHeader)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/deploy", nil))

	if seen == "" {
		t.Fatal("handler must see a generated request id")
	}
	if got := rec.Header().Get(requestIDHeader); got != seen {
		t.Fatalf("response id %q must match request id %q", got, seen)
	}
}

func TestRequestIDMiddlewareKeepsCallerID(t *testing.T) {
	var seen string
	h := requestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get(requestIDHeader)
	}))
	req := httptest.NewRequest("POST", "/deploy", nil)
	req.Header.Set(requestIDHeader, "caller-supplied-42")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen != "caller-supplied-42" {
		t.Fatalf("caller id must be kept, got %q", seen)
	}
}

func TestRequestIDMiddlewareRejectsHostileID(t *testing.T) {
	var seen string
	h := requestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get(requestIDHeader)
	}))
	req := httptest.NewRequest("POST", "/deploy", nil)
	req.Header.Set(requestIDHeader, `evil"id{}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen == `evil"id{}` || seen == "" {
		t.Fatalf("hostile id must be replaced with a generated one, got %q", seen)
	}
}

func TestSanitizeRequestID(t *testing.T) {
	cases := map[string]string{
		"abc-DEF_123.456": "abc-DEF_123.456",
		"  padded  ":      "padded",
		"has space":       "",
		"new\nline":       "",
	}
	for in, want := range cases {
		if got := sanitizeRequestID(in); got != want {
			t.Errorf("sanitizeRequestID(%q) = %q, want %q", in, got, want)
		}
	}
	long := strings64() + strings64()
	if got := sanitizeRequestID(long); len(got) != 64 {
		t.Errorf("long id must be capped at 64, got len %d", len(got))
	}
}

func strings64() string {
	s := ""
	for range 8 {
		s += "abcdefgh"
	}
	return s
}
