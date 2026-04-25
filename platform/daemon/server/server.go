// Package server provides the runfabricd HTTP server.
// It is independent of CLI flag parsing and cobra — the daemon command
// builds an Options value and calls New(opts).Start().
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/runfabric/runfabric/platform/daemon/configapi"
	runfabricruntime "github.com/runfabric/runfabric/platform/extensions/registry/loader/runtime"
	"github.com/runfabric/runfabric/platform/observability/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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
		opts.Address = "0.0.0.0"
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
// standard routes are registered; pass nil when no extra routes are needed.
func (s *Server) Handler(extraRoutes func(*http.ServeMux)) http.Handler {
	configSrv := configapi.NewServer(s.opts.Stage)
	configSrv.APIKey = s.opts.APIKey
	configSrv.RateLimitN = s.opts.RateLimit
	apiHandler := configSrv.Handler()

	if s.cache != nil {
		apiHandler = apiCacheMiddleware(s.cache, s.opts.Stage, apiHandler)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"version":  runfabricruntime.Version,
			"protocol": runfabricruntime.ProtocolVersion,
		})
	})
	mux.HandleFunc("POST /validate", apiHandler.ServeHTTP)
	mux.HandleFunc("POST /resolve", apiHandler.ServeHTTP)
	mux.HandleFunc("POST /plan", apiHandler.ServeHTTP)
	mux.HandleFunc("POST /deploy", apiHandler.ServeHTTP)
	mux.HandleFunc("POST /remove", apiHandler.ServeHTTP)
	mux.HandleFunc("POST /releases", apiHandler.ServeHTTP)

	if extraRoutes != nil {
		extraRoutes(mux)
	}

	return otelMiddleware(telemetry.Tracer("runfabric/daemon"), mux)
}

// otelMiddleware creates a span per request when OpenTelemetry is configured.
func otelMiddleware(tr trace.Tracer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tr.Start(r.Context(), r.Method+" "+r.URL.Path)
		defer span.End()
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.route", r.URL.Path),
		)
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
