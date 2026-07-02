package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultLatencyBuckets are seconds-based histogram buckets suitable for HTTP
// request durations (1ms .. 10s).
var DefaultLatencyBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

const (
	metricRequestsTotal   = "runfabric_http_requests_total"
	metricRequestDuration = "runfabric_http_request_duration_seconds"
	metricInflight        = "runfabric_http_requests_in_flight"
)

// HTTPMiddleware records RED metrics (Rate, Errors, Duration) for each request
// into reg (Default when nil). route should be a low-cardinality template
// (e.g. "/deploy"), never the raw path with IDs, to avoid label explosion.
func HTTPMiddleware(reg *Registry, next http.Handler) http.Handler {
	if reg == nil {
		reg = Default
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reg.AddGauge(metricInflight, "In-flight HTTP requests.", nil, 1)
		defer reg.AddGauge(metricInflight, "In-flight HTTP requests.", nil, -1)

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		labels := map[string]string{
			"method": r.Method,
			"route":  routeOf(r),
			"status": strconv.Itoa(rec.status),
		}
		reg.IncCounter(metricRequestsTotal, "Total HTTP requests by method, route and status.", labels)
		reg.Observe(metricRequestDuration, "HTTP request duration in seconds.", DefaultLatencyBuckets,
			map[string]string{"method": r.Method, "route": routeOf(r)}, time.Since(start).Seconds())
	})
}

// routeOf returns the matched pattern when the Go 1.22+ ServeMux set one,
// falling back to the cleaned path. Using the pattern keeps label cardinality
// bounded.
func routeOf(r *http.Request) string {
	if p := r.Pattern; p != "" {
		// Pattern looks like "POST /deploy"; keep just the path portion.
		if i := strings.IndexByte(p, ' '); i >= 0 && i+1 < len(p) {
			return p[i+1:]
		}
		return p
	}
	if r.URL != nil && r.URL.Path != "" {
		return r.URL.Path
	}
	return "unknown"
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.wroteHeader = true
	}
	return s.ResponseWriter.Write(b)
}
