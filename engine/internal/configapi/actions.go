package configapi

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/runfabric/runfabric/engine/internal/app"
)

// withTempConfig runs fn with a temp dir containing runfabric.yml from body, then removes the dir.
// configPath will be tempDir/runfabric.yml.
func withTempConfig(body []byte, stage string, fn func(configPath, stage string) (any, error)) (any, error) {
	dir, err := os.MkdirTemp("", "runfabric-api-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	configPath := filepath.Join(dir, "runfabric.yml")
	if err := os.WriteFile(configPath, body, 0600); err != nil {
		return nil, err
	}
	return fn(configPath, stage)
}

func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
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

	result, err := withTempConfig(body, stage, func(configPath, st string) (any, error) {
		return app.Plan(configPath, st, "")
	})
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stage": stage, "result": result})
}

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
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

	result, err := withTempConfig(body, stage, func(configPath, st string) (any, error) {
		return app.Deploy(configPath, st, "", false, false, nil, "")
	})
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stage": stage, "result": result})
}

func (s *Server) handleRemove(w http.ResponseWriter, r *http.Request) {
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

	result, err := withTempConfig(body, stage, func(configPath, st string) (any, error) {
		return app.Remove(configPath, st, "")
	})
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stage": stage, "result": result})
}

func (s *Server) handleReleases(w http.ResponseWriter, r *http.Request) {
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

	data, err := withTempConfig(body, stage, func(configPath, st string) (any, error) {
		return app.Dashboard(configPath, st)
	})
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	dd, _ := data.(*app.DashboardData)
	if dd == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stage": stage, "releases": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stage": stage, "service": dd.Service, "releases": dd.Stages})
}
