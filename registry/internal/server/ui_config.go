package server

import (
	"net/http"
	"strings"
)

const (
	defaultUIAuthURL = "https://auth.runfabric.cloud/device"
	defaultUIDocsURL = "https://runfabric.cloud/docs"
)

func (s *Server) handleUIConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"authLoginURL": normalizeURLOrDefault(s.uiAuthURL, defaultUIAuthURL),
		"cliDocsURL":   normalizeURLOrDefault(s.uiDocsURL, defaultUIDocsURL),
		"oidcIssuer":   strings.TrimSpace(s.oidcIssuer),
	})
	s.audit(r, "ui_config", "ok", nil)
}

func normalizeURLOrDefault(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "http://") {
		return trimmed
	}
	return fallback
}
