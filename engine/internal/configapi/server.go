package configapi

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"
)

const defaultStage = "dev"

// Server serves the YAML Configuration API: POST /validate, POST /resolve, POST /plan, POST /deploy, POST /remove, POST /releases.
type Server struct {
	Stage           string // default stage when not in query (e.g. "dev")
	APIKey          string // optional: require X-API-Key header when non-empty
	RateLimitN      int    // optional: max requests per RateLimitWindow per client (0 = disabled)
	RateLimitWindow time.Duration
}

// NewServer returns a config API server with optional default stage.
func NewServer(defaultStageIfEmpty string) *Server {
	if defaultStageIfEmpty == "" {
		defaultStageIfEmpty = defaultStage
	}
	return &Server{Stage: defaultStageIfEmpty}
}

// Handler returns the http.Handler for the config API. When APIKey or RateLimitN are set, requests are wrapped with auth and rate limiting.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /validate", s.handleValidate)
	mux.HandleFunc("POST /resolve", s.handleResolve)
	mux.HandleFunc("POST /plan", s.handlePlan)
	mux.HandleFunc("POST /deploy", s.handleDeploy)
	mux.HandleFunc("POST /remove", s.handleRemove)
	mux.HandleFunc("POST /releases", s.handleReleases)
	h := http.Handler(mux)
	if s.APIKey != "" {
		h = RequireAPIKey(s.APIKey)(h)
	}
	if s.RateLimitN > 0 {
		window := s.RateLimitWindow
		if window <= 0 {
			window = time.Minute
		}
		h = RateLimit(s.RateLimitN, window, func(r *http.Request) string { return r.RemoteAddr })(h)
	}
	return h
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "method not allowed"})
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = r.Body.Close()

	cfg, err := config.LoadFromBytes(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if err := config.Validate(cfg); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "method not allowed"})
		return
	}
	stage := r.URL.Query().Get("stage")
	if stage == "" {
		stage = s.Stage
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = r.Body.Close()

	cfg, err := config.LoadFromBytes(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if err := config.Validate(cfg); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	resolved, err := config.Resolve(cfg, stage)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stage": stage, "config": resolved})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
