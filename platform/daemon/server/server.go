// Package server provides the runfabricd HTTP server.
// It is independent of CLI flag parsing and cobra — the daemon command
// builds an Options value and calls New(opts).Start().
package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/runfabric/runfabric/platform/daemon/configapi"
	runfabricruntime "github.com/runfabric/runfabric/platform/extensions/registry/loader/runtime"
	"github.com/runfabric/runfabric/platform/observability/metrics"
	"github.com/runfabric/runfabric/platform/observability/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Options configures the daemon server. All fields have sensible zero-value defaults.
type Options struct {
	Address   string
	Port      int
	Stage     string
	APIKey    string
	RateLimit int
	CacheURL  string
	CacheTTL  time.Duration
}

// Server is the runfabricd HTTP server.
type Server struct {
	opts  Options
	cache *apiCache
}

// New creates a Server with resolved defaults.
func New(opts Options) *Server {
	if opts.Address == "" {
		// Default to loopback: the daemon exposes an unauthenticated-by-default
		// deploy API, so it must not bind all interfaces unless the operator opts
		// in (and supplies an API key — enforced by RequireAuthForBind).
		opts.Address = "127.0.0.1"
	}
	if opts.Port == 0 {
		opts.Port = 8766
	}
	if opts.Stage == "" {
		opts.Stage = "dev"
	}

	var cache *apiCache
	cacheURL := strings.TrimSpace(opts.CacheURL)
	if cacheURL == "" {
		cacheURL = os.Getenv("RUNFABRIC_DAEMON_CACHE_URL")
	}
	if isRedisURL(cacheURL) {
		ttl := opts.CacheTTL
		if ttl <= 0 {
			ttl = 5 * time.Minute
		}
		cache = newAPICache(cacheURL, ttl)
	}

	// Process/runtime gauges + build info on /metrics (idempotent registration).
	metrics.EnableRuntimeMetrics(nil, runfabricruntime.Version)

	return &Server{opts: opts, cache: cache}
}

// Addr returns the TCP address the server listens on.
func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.opts.Address, s.opts.Port)
}

// UsingCache returns true when a Redis cache is configured and connected.
func (s *Server) UsingCache() bool { return s.cache != nil }

// InvalidateStage purges cached responses for the given stage (call after deploy/remove).
func (s *Server) InvalidateStage(stage string) {
	if s.cache != nil {
		s.cache.invalidateStage(stage)
	}
}

// Handler builds the HTTP handler. extraRoutes is called with the mux after the
// standard routes are registered, along with an authorize middleware that
// applies the same API-key/rate-limit checks as the config API; mutating extra
// routes must be wrapped with it so they cannot bypass auth. Pass nil when no
// extra routes are needed.
func (s *Server) Handler(extraRoutes func(mux *http.ServeMux, authorize func(http.HandlerFunc) http.HandlerFunc)) http.Handler {
	configSrv := configapi.NewServer(s.opts.Stage)
	configSrv.APIKey = s.opts.APIKey
	configSrv.RateLimitN = s.opts.RateLimit
	apiHandler := configSrv.Handler()

	if s.cache != nil {
		apiHandler = apiCacheMiddleware(s.cache, s.opts.Stage, apiHandler)
	}

	mux := http.NewServeMux()
	// Liveness: the process is up. Must not depend on external systems.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// Readiness: dependencies are reachable. When a Redis cache is configured,
	// ping it so a broken cache takes the instance out of the load-balancer
	// rotation instead of serving errors.
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if s.cache != nil {
			if err := s.cache.ping(r.Context()); err != nil {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("cache unavailable: " + err.Error()))
				return
			}
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"version":  runfabricruntime.Version,
			"protocol": runfabricruntime.ProtocolVersion,
		})
	})
	// Prometheus scrape endpoint (RED metrics recorded by the middleware below).
	mux.Handle("GET /metrics", metrics.Handler())
	mux.HandleFunc("POST /validate", apiHandler.ServeHTTP)
	mux.HandleFunc("POST /resolve", apiHandler.ServeHTTP)
	mux.HandleFunc("POST /plan", apiHandler.ServeHTTP)
	mux.HandleFunc("POST /deploy", apiHandler.ServeHTTP)
	mux.HandleFunc("POST /remove", apiHandler.ServeHTTP)
	mux.HandleFunc("POST /releases", apiHandler.ServeHTTP)

	if extraRoutes != nil {
		extraRoutes(mux, configSrv.Authorize)
	}

	// Chain (outermost first): request-id → access log → RED metrics → span →
	// body cap → routes. Request id runs first so every later layer (log line,
	// span attribute, error response) sees the same correlation id.
	var handler http.Handler = metrics.HTTPMiddleware(nil,
		otelMiddleware(telemetry.Tracer("runfabric/daemon"),
			maxBodyMiddleware(maxRequestBody, mux)))
	if accessLogEnabled() {
		handler = accessLogMiddleware(handler)
	}
	return requestIDMiddleware(handler)
}

// maxRequestBody caps any single request body the daemon will read into memory.
const maxRequestBody = 8 << 20 // 8 MiB

// maxBodyMiddleware bounds request bodies so a large or slow upload cannot
// exhaust memory. Handlers that read the body see io.EOF past the limit.
func maxBodyMiddleware(limit int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}

// NewHTTPServer builds an *http.Server with hardened timeouts for the given
// handler. Callers own Start/Shutdown so they can drain in-flight requests.
func (s *Server) NewHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              s.Addr(),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// RequireAuthForBind returns an error when binding the given address would expose
// the unauthenticated daemon API beyond loopback. A non-loopback bind without an
// API key is refused so the deploy API is never reachable from the network
// without authentication.
func RequireAuthForBind(address, apiKey string) error {
	if apiKey != "" || IsLoopbackHost(address) {
		return nil
	}
	return fmt.Errorf("refusing to bind %q without --api-key: a non-loopback address exposes the deploy API unauthenticated; set --api-key or bind a loopback address (127.0.0.1)", address)
}

// IsLoopbackHost reports whether host (an address without port) is loopback.
func IsLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == "" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// traceIDHeader exposes the request's trace id to callers and to the access
// log so logs and traces correlate even without a tracing backend attached.
const traceIDHeader = "X-Trace-Id"

// tracePropagation extracts W3C traceparent/tracestate from inbound requests.
// Explicit (not the global propagator) so extraction works deterministically
// even when telemetry.Init has not run.
var tracePropagation = propagation.TraceContext{}

// otelMiddleware creates a span per request when OpenTelemetry is configured.
// An inbound traceparent header is honored: the span joins the caller's trace
// (same trace id) instead of starting a new root, so a deploy triggered by an
// upstream service maps end-to-end in the tracing backend.
func otelMiddleware(tr trace.Tracer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := tracePropagation.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := tr.Start(ctx, r.Method+" "+r.URL.Path, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()
		sc := span.SpanContext()
		if !sc.HasTraceID() {
			sc = trace.SpanContextFromContext(ctx)
		}
		if sc.HasTraceID() {
			id := sc.TraceID().String()
			w.Header().Set(traceIDHeader, id)
			// Mirror onto the request so the access-log layer (outside this
			// middleware) can include trace_id in its line.
			r.Header.Set(traceIDHeader, id)
		}
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.route", r.URL.Path),
		)
		if id := r.Header.Get(requestIDHeader); id != "" {
			span.SetAttributes(attribute.String("http.request_id", id))
		}
		r = r.WithContext(ctx)
		rec := &otelResponseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if rec.status >= 400 {
			span.SetStatus(codes.Error, http.StatusText(rec.status))
			span.SetAttributes(attribute.Int("http.status_code", rec.status))
		}
	})
}

type otelResponseRecorder struct {
	http.ResponseWriter
	status int
}

func (o *otelResponseRecorder) WriteHeader(code int) {
	o.status = code
	o.ResponseWriter.WriteHeader(code)
}

func isRedisURL(url string) bool {
	return strings.HasPrefix(url, "redis://") || strings.HasPrefix(url, "rediss://")
}
