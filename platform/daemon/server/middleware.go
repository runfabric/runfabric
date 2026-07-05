package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// requestIDHeader is the header the daemon reads/echoes for request correlation.
const requestIDHeader = "X-Request-Id"

// requestIDMiddleware ensures every request carries a request id: an incoming
// X-Request-Id is kept (sanitized), otherwise one is generated. The id is set
// on the request (so downstream middleware, spans and logs can read it) and
// echoed on the response so callers can correlate.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := sanitizeRequestID(r.Header.Get(requestIDHeader))
		if id == "" {
			id = newRequestID()
		}
		r.Header.Set(requestIDHeader, id)
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r)
	})
}

// sanitizeRequestID bounds caller-supplied ids so a hostile value cannot
// smuggle newlines into logs or blow up label/attribute sizes.
func sanitizeRequestID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) > 64 {
		id = id[:64]
	}
	for _, c := range id {
		valid := c == '-' || c == '_' || c == '.' ||
			(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if !valid {
			return ""
		}
	}
	return id
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}

// accessLogEnabled reports whether request logging is on. Enabled by default;
// RUNFABRIC_DAEMON_ACCESS_LOG=0/false turns it off.
func accessLogEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("RUNFABRIC_DAEMON_ACCESS_LOG"))) {
	case "0", "false", "no", "off":
		return false
	}
	return true
}

// accessLogMiddleware emits one structured JSON line per API request to stdout.
// Health/readiness/metrics probes are skipped — they fire every few seconds and
// would drown real traffic.
func accessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", "/readyz", "/metrics":
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &accessLogRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		remote := r.RemoteAddr
		if host, _, err := net.SplitHostPort(remote); err == nil {
			remote = host
		}
		entry := map[string]any{
			"time":        time.Now().UTC().Format(time.RFC3339Nano),
			"msg":         "http_request",
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      rec.status,
			"duration_ms": time.Since(start).Milliseconds(),
			"bytes":       rec.bytes,
			"request_id":  r.Header.Get(requestIDHeader),
			"remote":      remote,
		}
		// Set by the span layer when tracing context exists (inbound or new).
		if traceID := r.Header.Get(traceIDHeader); traceID != "" {
			entry["trace_id"] = traceID
		}
		line, err := json.Marshal(entry)
		if err != nil {
			return
		}
		_, _ = os.Stdout.Write(append(line, '\n'))
	})
}

type accessLogRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (a *accessLogRecorder) WriteHeader(code int) {
	a.status = code
	a.ResponseWriter.WriteHeader(code)
}

func (a *accessLogRecorder) Write(b []byte) (int, error) {
	n, err := a.ResponseWriter.Write(b)
	a.bytes += n
	return n, err
}
