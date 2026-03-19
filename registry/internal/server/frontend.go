package server

import (
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func (s *Server) handleFrontendOrNotFound(w http.ResponseWriter, r *http.Request) {
	if s.tryServeFrontend(w, r) {
		return
	}
	s.handleNotFound(w, r)
}

func (s *Server) tryServeFrontend(w http.ResponseWriter, r *http.Request) bool {
	if !s.webEnabled {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if isAPIPath(r.URL.Path) {
		return false
	}

	cleanPath := path.Clean("/" + strings.TrimSpace(r.URL.Path))
	if cleanPath == "/" {
		s.serveIndex(w, r)
		return true
	}

	webRelative := strings.TrimPrefix(cleanPath, "/")
	if webRelative == "" {
		s.serveIndex(w, r)
		return true
	}

	fullPath := filepath.Join(s.webDir, filepath.FromSlash(webRelative))
	if fileInfo, err := os.Stat(fullPath); err == nil && !fileInfo.IsDir() {
		s.setCacheHeaders(w, webRelative)
		http.ServeFile(w, r, fullPath)
		return true
	}

	// Missing static asset should remain a 404, but route-like paths fall back to SPA index.
	if strings.Contains(path.Base(cleanPath), ".") {
		return false
	}

	s.serveIndex(w, r)
	return true
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, s.webIndexPath)
}

func (s *Server) setCacheHeaders(w http.ResponseWriter, webRelative string) {
	lower := strings.ToLower(webRelative)
	if strings.HasPrefix(lower, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		if ctype := mime.TypeByExtension(filepath.Ext(lower)); ctype != "" {
			w.Header().Set("Content-Type", ctype)
		}
		return
	}
	if strings.HasSuffix(lower, ".json") {
		w.Header().Set("Cache-Control", "public, max-age=300")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
}

func isAPIPath(pathname string) bool {
	path := strings.TrimSpace(pathname)
	switch {
	case path == "/healthz", path == "/ready", path == "/v1", path == "/packages", path == "/artifacts", path == "/bin":
		return true
	case strings.HasPrefix(path, "/v1/"), strings.HasPrefix(path, "/packages/"), strings.HasPrefix(path, "/artifacts/"):
		return true
	default:
		return false
	}
}
