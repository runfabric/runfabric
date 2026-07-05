package configapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// maxInvokePayload caps the /invoke request body so a standalone config API
// (no outer max-body middleware) cannot be made to buffer unbounded input.
const maxInvokePayload = 4 << 20 // 4 MiB

// runOp executes fn under the per-request credential + serialization path
// (every op here Bootstraps, so it takes the same deployMu + credential env
// semantics as deploy) and writes the raw JSON payload.
func (s *Server) runOp(w http.ResponseWriter, r *http.Request, metric string, fn func(cfgPath, stage string) (json.RawMessage, error)) {
	cfgPath, err := configPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var payload json.RawMessage
	start := time.Now()
	err = s.withProviderCreds(r, func() error {
		var oerr error
		payload, oerr = fn(cfgPath, s.stage(r))
		return oerr
	})
	observeOperation(metric, start, err)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeRawOK(w, payload)
}

// boolParam reads a query flag ("1"/"true"/"yes" = true).
func boolParam(r *http.Request, name string) bool {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get(name))) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// relParam returns a workspace-confined relative path query param ("" if
// unset). Same confinement as configPath: no absolute paths, no "../" escapes.
func relParam(r *http.Request, name string) (string, error) {
	p := strings.TrimSpace(r.URL.Query().Get(name))
	if p == "" {
		return "", nil
	}
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("%s path must be relative to the workspace", name)
	}
	clean := filepath.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s path escapes the workspace", name)
	}
	return clean, nil
}

// handleFabricHealth probes every recorded fabric endpoint (multi-cloud) and
// returns the fabric state with per-endpoint health.
func (s *Server) handleFabricHealth(w http.ResponseWriter, r *http.Request) {
	s.runOp(w, r, "fabric_health", func(cfgPath, stage string) (json.RawMessage, error) {
		return s.core.FabricHealth(cfgPath, stage)
	})
}

// handleFabricTargets lists the config's fabric.targets provider keys.
func (s *Server) handleFabricTargets(w http.ResponseWriter, r *http.Request) {
	s.runOp(w, r, "fabric_targets", func(cfgPath, stage string) (json.RawMessage, error) {
		return s.core.FabricTargets(cfgPath, stage)
	})
}

// handleInvoke calls one deployed function (or `workflow:<name>` orchestration
// target) with the request body as JSON payload.
func (s *Server) handleInvoke(w http.ResponseWriter, r *http.Request) {
	function := strings.TrimSpace(r.URL.Query().Get("function"))
	if function == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("function query parameter is required"))
		return
	}
	payload, err := io.ReadAll(io.LimitReader(r.Body, maxInvokePayload+1))
	_ = r.Body.Close()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if len(payload) > maxInvokePayload {
		writeErr(w, http.StatusRequestEntityTooLarge, fmt.Errorf("invoke payload too large"))
		return
	}
	s.runOp(w, r, "invoke", func(cfgPath, stage string) (json.RawMessage, error) {
		return s.core.Invoke(cfgPath, stage, function, provider(r), payload)
	})
}

// handleLogs returns provider + local logs for one function ("" = all).
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	s.runOp(w, r, "logs", func(cfgPath, stage string) (json.RawMessage, error) {
		return s.core.Logs(cfgPath, stage,
			strings.TrimSpace(r.URL.Query().Get("function")),
			provider(r),
			strings.TrimSpace(r.URL.Query().Get("service")))
	})
}

// handleFunctionMetrics returns per-function metrics from the provider (the
// daemon's own Prometheus metrics stay on GET /metrics).
func (s *Server) handleFunctionMetrics(w http.ResponseWriter, r *http.Request) {
	s.runOp(w, r, "function_metrics", func(cfgPath, stage string) (json.RawMessage, error) {
		return s.core.FunctionMetrics(cfgPath, stage, provider(r),
			strings.TrimSpace(r.URL.Query().Get("service")),
			boolParam(r, "all"))
	})
}

// handleTraces returns traces aggregated by service/stage from the provider.
func (s *Server) handleTraces(w http.ResponseWriter, r *http.Request) {
	s.runOp(w, r, "traces", func(cfgPath, stage string) (json.RawMessage, error) {
		return s.core.Traces(cfgPath, stage, provider(r),
			strings.TrimSpace(r.URL.Query().Get("service")),
			boolParam(r, "all"))
	})
}

// handleDoctor runs backend + provider readiness checks.
func (s *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	s.runOp(w, r, "doctor", func(cfgPath, stage string) (json.RawMessage, error) {
		return s.core.Doctor(cfgPath, stage, provider(r))
	})
}

// handleRecover replays/rolls back an unfinished transaction journal.
// mode=rollback|resume|inspect (default rollback); dryRun=1 previews.
func (s *Server) handleRecover(w http.ResponseWriter, r *http.Request) {
	mode := strings.TrimSpace(r.URL.Query().Get("mode"))
	if mode == "" {
		mode = "rollback"
	}
	dryRun := boolParam(r, "dryRun")
	s.runOp(w, r, "recover", func(cfgPath, stage string) (json.RawMessage, error) {
		return s.core.Recover(cfgPath, stage, mode, dryRun)
	})
}

// handleStateOp runs a state-backend operation:
// POST /state/{list|pull|backup|restore|reconcile|migrate|unlock|lock-steal}.
// Path params (out, file) are confined to the daemon workspace.
func (s *Server) handleStateOp(w http.ResponseWriter, r *http.Request) {
	op := r.PathValue("op")
	out, err := relParam(r, "out")
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	file, err := relParam(r, "file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	params := map[string]string{
		"out":  out,
		"file": file,
		"from": strings.TrimSpace(r.URL.Query().Get("from")),
		"to":   strings.TrimSpace(r.URL.Query().Get("to")),
	}
	if boolParam(r, "force") {
		params["force"] = "true"
	}
	s.runOp(w, r, "state_"+strings.ReplaceAll(op, "-", "_"), func(cfgPath, stage string) (json.RawMessage, error) {
		return s.core.StateOp(op, cfgPath, stage, params)
	})
}

// handleRouterOp runs a router operation over the recorded fabric state:
// POST /router/{history|simulate|verify|shift|restore}. (/router/sync keeps
// its dedicated handler.)
func (s *Server) handleRouterOp(w http.ResponseWriter, r *http.Request) {
	op := r.PathValue("op")
	params := map[string]string{
		"requests": strings.TrimSpace(r.URL.Query().Get("requests")),
		"down":     strings.TrimSpace(r.URL.Query().Get("down")),
		"window":   strings.TrimSpace(r.URL.Query().Get("window")),
		"provider": strings.TrimSpace(r.URL.Query().Get("provider")),
		"percent":  strings.TrimSpace(r.URL.Query().Get("percent")),
		"snapshot": strings.TrimSpace(r.URL.Query().Get("snapshot")),
	}
	if boolParam(r, "latest") {
		params["latest"] = "true"
	}
	if boolParam(r, "dryRun") {
		params["dryRun"] = "true"
	}
	s.runOp(w, r, "router_"+strings.ReplaceAll(op, "-", "_"), func(cfgPath, stage string) (json.RawMessage, error) {
		return s.core.RouterOp(op, cfgPath, stage, params)
	})
}
