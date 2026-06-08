package configapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Rate limiting must key on client IP, not host:port. A caller using many
// ephemeral source ports from the same IP must not get a fresh bucket per
// connection (which would defeat the limit entirely).
func TestRateLimitKeysByIPNotPort(t *testing.T) {
	s := NewServer("dev")
	s.RateLimitN = 2

	call := func(remoteAddr string) int {
		r := httptest.NewRequest(http.MethodPost, "/validate", nil)
		r.RemoteAddr = remoteAddr
		w := httptest.NewRecorder()
		_ = s.authorizeAndLimit(w, r)
		return w.Code
	}

	if code := call("203.0.113.5:1001"); code != http.StatusOK {
		t.Fatalf("request 1: got %d, want 200", code)
	}
	if code := call("203.0.113.5:1002"); code != http.StatusOK {
		t.Fatalf("request 2 (new port): got %d, want 200", code)
	}
	// Third request from the same IP (different port) must be limited.
	if code := call("203.0.113.5:1003"); code != http.StatusTooManyRequests {
		t.Fatalf("request 3 (new port, same IP): got %d, want 429", code)
	}
	// A different IP gets its own bucket.
	if code := call("198.51.100.7:2000"); code != http.StatusOK {
		t.Fatalf("different IP: got %d, want 200", code)
	}
}

func TestConstantTimeAPIKey(t *testing.T) {
	s := NewServer("dev")
	s.APIKey = "secret-key"

	do := func(key string) int {
		r := httptest.NewRequest(http.MethodPost, "/validate", nil)
		r.RemoteAddr = "203.0.113.9:5000"
		if key != "" {
			r.Header.Set("X-API-Key", key)
		}
		w := httptest.NewRecorder()
		_ = s.authorizeAndLimit(w, r)
		return w.Code
	}

	if code := do(""); code != http.StatusUnauthorized {
		t.Fatalf("missing key: got %d, want 401", code)
	}
	if code := do("wrong"); code != http.StatusUnauthorized {
		t.Fatalf("wrong key: got %d, want 401", code)
	}
	if code := do("secret-key"); code != http.StatusOK {
		t.Fatalf("correct key: got %d, want 200", code)
	}
}
