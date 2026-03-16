package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// HTTPHandler wraps a Handler to implement http.Handler (for use with net/http or any HTTP framework).
func HTTPHandler(h Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		var event map[string]any
		if len(body) > 0 {
			if err := json.Unmarshal(body, &event); err != nil {
				event = map[string]any{"body": string(body)}
			}
		}
		if event == nil {
			event = make(map[string]any)
		}
		runCtx := &Context{
			Stage:        r.URL.Query().Get("stage"),
			FunctionName: r.Header.Get("X-Runfabric-Function"),
			RequestID:    r.Header.Get("X-Request-Id"),
		}
		if runCtx.Stage == "" {
			runCtx.Stage = "dev"
		}
		resp, err := h(context.Background(), event, runCtx)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
