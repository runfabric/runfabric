package configapi

import (
	"net/http"
	"sync"
	"time"
)

// RequireAPIKey returns a middleware that checks X-API-Key header when key is non-empty. When key is empty, no check is done.
func RequireAPIKey(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			if r.Header.Get("X-API-Key") != key {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "missing or invalid X-API-Key"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit returns a middleware that limits each key (e.g. IP) to maxRequests per window. keyFn extracts the key from the request (e.g. r.RemoteAddr).
func RateLimit(maxRequests int, window time.Duration, keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	type entry struct {
		count  int
		window time.Time
	}
	var mu sync.Mutex
	store := make(map[string]*entry)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if maxRequests <= 0 || keyFn == nil {
				next.ServeHTTP(w, r)
				return
			}
			key := keyFn(r)
			if key == "" {
				key = r.RemoteAddr
			}
			mu.Lock()
			e := store[key]
			now := time.Now()
			if e == nil || now.Sub(e.window) > window {
				e = &entry{count: 1, window: now}
				store[key] = e
			} else {
				e.count++
				if e.count > maxRequests {
					mu.Unlock()
					writeJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "error": "rate limit exceeded"})
					return
				}
			}
			mu.Unlock()
			next.ServeHTTP(w, r)
		})
	}
}
