package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHasCredentialHeaders(t *testing.T) {
	cases := map[string]bool{
		"X-Provider-Aws-Access-Key-Id": true,
		"X-State-Postgres-Url":         true,
		"X-Router-Api-Token":           true,
		"X-Secret-Vault-Token":         true,
		"X-API-Key":                    false,
		"X-Request-Id":                 false,
		"Content-Type":                 false,
	}
	for name, want := range cases {
		h := http.Header{}
		h.Set(name, "v")
		if got := hasCredentialHeaders(h); got != want {
			t.Errorf("hasCredentialHeaders(%s) = %v, want %v", name, got, want)
		}
	}
}

func TestCacheMiddlewareBypassesCredentialedRequests(t *testing.T) {
	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"plan":"fresh"}`))
	})
	// nil *apiCache: the middleware must still route credentialed requests
	// straight through the handler (and never panic on cache access).
	h := apiCacheMiddleware(nil, "dev", next)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/plan?stage=dev", nil)
		req.Header.Set("X-Router-Api-Token", "tenant-token")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
	}
	if calls != 2 {
		t.Fatalf("handler calls = %d, want 2 (credentialed requests must never be served from cache)", calls)
	}
}
