package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maxArtifactBytes = 256 * 1024 * 1024

func (s *Server) handleArtifacts(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/artifacts/"))
	if key == "" {
		s.handleNotFound(w, r)
		return
	}
	if strings.Contains(key, "..") {
		writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "INVALID_REQUEST", Message: "invalid artifact key", RequestID: requestIDFromRequest(r)})
		return
	}
	if err := s.verifyArtifactToken(r, key); err != nil {
		s.unauthorized(w, r, err.Error())
		return
	}
	switch r.Method {
	case http.MethodPut:
		body, err := io.ReadAll(io.LimitReader(r.Body, maxArtifactBytes))
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "INVALID_REQUEST", Message: "failed reading artifact body", Details: map[string]any{"cause": err.Error()}, RequestID: requestIDFromRequest(r)})
			return
		}
		path, err := s.artifactPath(key)
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "INVALID_REQUEST", Message: err.Error(), RequestID: requestIDFromRequest(r)})
			return
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, apiError{Code: "INTERNAL", Message: "failed to create artifact directory", Details: map[string]any{"cause": err.Error()}, RequestID: requestIDFromRequest(r)})
			return
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, apiError{Code: "INTERNAL", Message: "failed to write artifact", Details: map[string]any{"cause": err.Error()}, RequestID: requestIDFromRequest(r)})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"uploaded": true, "key": key, "sizeBytes": len(body)})
	case http.MethodGet:
		path, err := s.artifactPath(key)
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, apiError{Code: "INVALID_REQUEST", Message: err.Error(), RequestID: requestIDFromRequest(r)})
			return
		}
		b, err := os.ReadFile(path)
		if err != nil {
			writeAPIError(w, r, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "artifact not found", Details: map[string]any{"key": key}, RequestID: requestIDFromRequest(r)})
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(b)))
		_, _ = w.Write(b)
	default:
		s.methodNotAllowed(w, r)
	}
}

func (s *Server) artifactPath(key string) (string, error) {
	base := filepath.Join(s.store.UploadsDir(), "artifacts")
	clean := filepath.Clean("/" + key)
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || clean == "." {
		return "", fmt.Errorf("invalid artifact key")
	}
	path := filepath.Join(base, clean)
	baseClean := filepath.Clean(base)
	if !strings.HasPrefix(path, baseClean+string(filepath.Separator)) && path != baseClean {
		return "", fmt.Errorf("invalid artifact key path")
	}
	return path, nil
}

func (s *Server) signArtifactToken(key, method, exp string) string {
	payload := strings.ToUpper(strings.TrimSpace(method)) + "|" + strings.TrimSpace(key) + "|" + strings.TrimSpace(exp)
	sum := sha256.Sum256([]byte(s.artifactSigningSecret + "|" + payload))
	return hex.EncodeToString(sum[:])
}

func (s *Server) verifyArtifactToken(r *http.Request, key string) error {
	exp := strings.TrimSpace(r.URL.Query().Get("exp"))
	sig := strings.TrimSpace(r.URL.Query().Get("sig"))
	method := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("method")))
	if exp == "" || sig == "" || method == "" {
		return fmt.Errorf("missing signed URL query parameters")
	}
	sec, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid signed URL expiry")
	}
	if time.Now().UTC().After(time.Unix(sec, 0).UTC()) {
		return fmt.Errorf("signed URL expired")
	}
	if method != strings.ToUpper(strings.TrimSpace(r.Method)) {
		return fmt.Errorf("signed URL method mismatch")
	}
	want := s.signArtifactToken(key, method, exp)
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(sig)), []byte(strings.ToLower(want))) != 1 {
		return fmt.Errorf("invalid signed URL signature")
	}
	return nil
}
