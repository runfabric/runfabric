package configapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Server provides a lightweight config API surface used by CLI daemon commands.
type Server struct {
	Stage      string
	APIKey     string
	RateLimitN int
	core       CoreWorkflowConnector

	mu       sync.Mutex
	requests map[string][]time.Time
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := s.authorizeAndLimit(w, r); err != nil {
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) authorizeAndLimit(w http.ResponseWriter, r *http.Request) error {
	if s.APIKey != "" && r.Header.Get("X-API-Key") != s.APIKey {
		writeErr(w, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return fmt.Errorf("unauthorized")
	}
	if s.RateLimitN <= 0 {
		return nil
	}
	ip := r.RemoteAddr
	now := time.Now()
	cutoff := now.Add(-1 * time.Minute)
	s.mu.Lock()
	defer s.mu.Unlock()
	hits := s.requests[ip]
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= s.RateLimitN {
		writeErr(w, http.StatusTooManyRequests, fmt.Errorf("rate limit exceeded"))
		return fmt.Errorf("rate limit exceeded")
	}
	s.requests[ip] = append(kept, now)
	return nil
}

func (s *Server) stage(r *http.Request) string {
	if st := r.URL.Query().Get("stage"); st != "" {
		return st
	}
	return s.Stage
}

func configPath(r *http.Request) string {
	if p := r.URL.Query().Get("config"); p != "" {
		return p
	}
	return "runfabric.yml"
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	err := s.core.Validate(configPath(r), s.stage(r))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeOK(w, map[string]any{"ok": true})
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.core.Resolve(configPath(r), s.stage(r))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeRawOK(w, cfg.Payload)
}

func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	res, err := s.core.Plan(configPath(r), s.stage(r))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeRawOK(w, res.Payload)
}

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	res, err := s.core.Deploy(configPath(r), s.stage(r))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeRawOK(w, res.Payload)
}

func (s *Server) handleRemove(w http.ResponseWriter, r *http.Request) {
	res, err := s.core.Remove(configPath(r), s.stage(r))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeRawOK(w, res.Payload)
}

func (s *Server) handleReleases(w http.ResponseWriter, r *http.Request) {
	res, err := s.core.Releases(configPath(r))
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
