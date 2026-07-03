package configapi

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// rateWindow is the sliding window over which RateLimitN requests are allowed.
const rateWindow = time.Minute

// Server provides a lightweight config API surface used by CLI daemon commands.
type Server struct {
	Stage      string
	APIKey     string
	RateLimitN int
	core       CoreWorkflowConnector

	mu          sync.Mutex
	requests    map[string][]time.Time
	lastSweepAt time.Time

	// deployMu serializes deploy/remove so per-request provider credentials can be
	// applied to the process env and restored without racing concurrent requests.
	deployMu sync.Mutex
}

func NewServer(stage string) *Server {
	if stage == "" {
		stage = "dev"
	}
	return &Server{Stage: stage, core: coreWorkflowAdapter{}, requests: make(map[string][]time.Time)}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /validate", s.handleValidate)
	mux.HandleFunc("POST /resolve", s.handleResolve)
	mux.HandleFunc("POST /plan", s.handlePlan)
	mux.HandleFunc("POST /deploy", s.handleDeploy)
	mux.HandleFunc("POST /remove", s.handleRemove)
	mux.HandleFunc("POST /releases", s.handleReleases)
	return s.Authorize(mux.ServeHTTP)
}

// Authorize wraps h with the same API-key and rate-limit checks the config API
// applies, so additional routes (e.g. daemon dashboard actions) registered
// outside Handler cannot bypass authentication.
func (s *Server) Authorize(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.authorizeAndLimit(w, r); err != nil {
			return
		}
		h(w, r)
	}
}

func (s *Server) authorizeAndLimit(w http.ResponseWriter, r *http.Request) error {
	// Constant-time comparison so the API key cannot be recovered via a timing
	// side-channel on the byte-by-byte short-circuit of ==/!=.
	if s.APIKey != "" {
		got := r.Header.Get("X-API-Key")
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.APIKey)) != 1 {
			writeErr(w, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
			return fmt.Errorf("unauthorized")
		}
	}
	if s.RateLimitN <= 0 {
		return nil
	}
	// Key by client IP (not host:port): the ephemeral source port differs per
	// TCP connection, so keying on RemoteAddr would give each connection its own
	// bucket and let a caller bypass the limit by opening new connections.
	ip := clientIP(r)
	now := time.Now()
	cutoff := now.Add(-rateWindow)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked(cutoff)
	hits := s.requests[ip]
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= s.RateLimitN {
		s.requests[ip] = kept
		writeErr(w, http.StatusTooManyRequests, fmt.Errorf("rate limit exceeded"))
		return fmt.Errorf("rate limit exceeded")
	}
	s.requests[ip] = append(kept, now)
	return nil
}

// clientIP extracts the client IP from RemoteAddr, dropping the ephemeral port.
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// sweepLocked evicts IP buckets whose newest request is older than cutoff so the
// requests map cannot grow without bound (a slow memory-exhaustion DoS). Runs at
// most once per window. Caller must hold s.mu.
func (s *Server) sweepLocked(cutoff time.Time) {
	now := time.Now()
	if now.Sub(s.lastSweepAt) < rateWindow {
		return
	}
	s.lastSweepAt = now
	for ip, hits := range s.requests {
		newest := time.Time{}
		for _, t := range hits {
			if t.After(newest) {
				newest = t
			}
		}
		if newest.Before(cutoff) || newest.IsZero() {
			delete(s.requests, ip)
		}
	}
}

func (s *Server) stage(r *http.Request) string {
	if st := r.URL.Query().Get("stage"); st != "" {
		return st
	}
	return s.Stage
}

// configPath resolves the requested config path, confining it to the workspace.
// An absolute path or a "../" escape is rejected so a caller cannot make the
// daemon read (and, via /resolve, return resolved secrets from) arbitrary files
// outside the workspace it serves.
func configPath(r *http.Request) (string, error) {
	p := r.URL.Query().Get("config")
	if p == "" {
		return "runfabric.yml", nil
	}
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("config path must be relative to the workspace")
	}
	clean := filepath.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("config path escapes the workspace")
	}
	return clean, nil
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	cfgPath, err := configPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.core.Validate(cfgPath, s.stage(r)); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeOK(w, map[string]any{"ok": true})
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	cfgPath, err := configPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := s.core.Resolve(cfgPath, s.stage(r))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeRawOK(w, cfg.Payload)
}

func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	cfgPath, err := configPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.core.Plan(cfgPath, s.stage(r))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeRawOK(w, res.Payload)
}

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	cfgPath, err := configPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var res *DeployResponse
	err = s.withProviderCreds(r, func() error {
		var derr error
		res, derr = s.core.Deploy(cfgPath, s.stage(r))
		return derr
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeRawOK(w, res.Payload)
}

func (s *Server) handleRemove(w http.ResponseWriter, r *http.Request) {
	cfgPath, err := configPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var res *RemoveResponse
	err = s.withProviderCreds(r, func() error {
		var rerr error
		res, rerr = s.core.Remove(cfgPath, s.stage(r))
		return rerr
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeRawOK(w, res.Payload)
}

// providerCredHeaders are per-request cloud credentials app-service forwards so a
// deploy runs against the project's OWN account, not the daemon's ambient creds.
var providerCredEnvByHeader = map[string]string{
	"X-Provider-Aws-Access-Key-Id":     "AWS_ACCESS_KEY_ID",
	"X-Provider-Aws-Secret-Access-Key": "AWS_SECRET_ACCESS_KEY",
	"X-Provider-Aws-Session-Token":     "AWS_SESSION_TOKEN",
	"X-Provider-Aws-Region":            "AWS_REGION",
}

// withProviderCreds applies any per-request provider credentials to the process
// env for the duration of fn, then restores the prior env. Serialized by deployMu
// so concurrent deploys never see each other's credentials.
func (s *Server) withProviderCreds(r *http.Request, fn func() error) error {
	creds := map[string]string{}
	for header, envKey := range providerCredEnvByHeader {
		if v := strings.TrimSpace(r.Header.Get(header)); v != "" {
			creds[envKey] = v
		}
	}
	if len(creds) == 0 {
		return fn()
	}
	// Mirror region to AWS_DEFAULT_REGION so the SDK default chain honors it.
	if region, ok := creds["AWS_REGION"]; ok {
		creds["AWS_DEFAULT_REGION"] = region
	}

	s.deployMu.Lock()
	defer s.deployMu.Unlock()

	// Snapshot the env keys we are about to touch (plus a clean slate for the
	// credential keys so a partial set never mixes with ambient creds).
	touched := []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "AWS_REGION", "AWS_DEFAULT_REGION"}
	type prev struct {
		val string
		set bool
	}
	saved := map[string]prev{}
	for _, k := range touched {
		v, ok := os.LookupEnv(k)
		saved[k] = prev{val: v, set: ok}
		_ = os.Unsetenv(k)
	}
	for k, v := range creds {
		_ = os.Setenv(k, v)
	}
	defer func() {
		for _, k := range touched {
			if p := saved[k]; p.set {
				_ = os.Setenv(k, p.val)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}()

	return fn()
}

func (s *Server) handleReleases(w http.ResponseWriter, r *http.Request) {
	cfgPath, err := configPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.core.Releases(cfgPath)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeRawOK(w, res.Payload)
}

func writeOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(data)
}

func writeRawOK(w http.ResponseWriter, payload json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if len(payload) == 0 {
		_, _ = w.Write([]byte("null"))
		return
	}
	_, _ = bytes.NewBuffer(payload).WriteTo(w)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
}
