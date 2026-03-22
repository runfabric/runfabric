package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (s *Server) readCachedJSON(w http.ResponseWriter, r *http.Request, scope string) bool {
	if s.cache == nil || strings.TrimSpace(scope) == "" {
		return false
	}
	key := s.cacheKey(r, scope)
	b, ok := s.cache.Get(key)
	if !ok {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
	return true
}

func (s *Server) writeAndCacheJSON(w http.ResponseWriter, r *http.Request, status int, payload any, scope string, ttl time.Duration) {
	if status >= 200 && status < 300 && strings.TrimSpace(scope) != "" && s.cache != nil {
		if b, err := json.Marshal(payload); err == nil {
			s.cache.Set(s.cacheKey(r, scope), b, ttl)
		}
	}
	writeJSON(w, status, payload)
}

func (s *Server) invalidateCacheScope() {
	s.cacheEpoch.Add(1)
}

func (s *Server) cacheKey(r *http.Request, scope string) string {
	epoch := s.cacheEpoch.Load()
	host := strings.TrimSpace(r.Host)
	rawQ := strings.TrimSpace(r.URL.RawQuery)
	if rawQ == "" {
		rawQ = "-"
	}
	authHash := "-"
	if auth := strings.TrimSpace(r.Header.Get("Authorization")); auth != "" {
		sum := sha256.Sum256([]byte(auth))
		authHash = hex.EncodeToString(sum[:8])
	}
	return fmt.Sprintf("v:%d|h:%s|s:%s|p:%s|q:%s|a:%s", epoch, host, scope, r.URL.Path, rawQ, authHash)
}
