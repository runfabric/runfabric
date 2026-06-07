package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAuthForBind(t *testing.T) {
	cases := []struct {
		address string
		apiKey  string
		wantErr bool
	}{
		{"127.0.0.1", "", false},     // loopback, no key — ok
		{"localhost", "", false},     // loopback name — ok
		{"::1", "", false},           // ipv6 loopback — ok
		{"0.0.0.0", "", true},        // all interfaces, no key — refused
		{"192.168.1.10", "", true},   // LAN, no key — refused
		{"0.0.0.0", "secret", false}, // non-loopback but key set — ok
	}
	for _, c := range cases {
		err := RequireAuthForBind(c.address, c.apiKey)
		if (err != nil) != c.wantErr {
			t.Errorf("RequireAuthForBind(%q, key=%q) err=%v, wantErr=%v", c.address, c.apiKey, err, c.wantErr)
		}
	}
}

// TestHandler_ExtraRoutesAuthorized verifies that an extra route wrapped with the
// provided authorize middleware enforces the API key — guarding the regression
// where dashboard /action/* routes bypassed auth.
func TestHandler_ExtraRoutesAuthorized(t *testing.T) {
	srv := New(Options{Address: "127.0.0.1", APIKey: "secret", Stage: "dev"})
	handler := srv.Handler(func(mux *http.ServeMux, authorize func(http.HandlerFunc) http.HandlerFunc) {
		mux.HandleFunc("POST /action/deploy", authorize(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("did-deploy"))
		}))
	})

	// No API key -> must be rejected, handler body must not run.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/action/deploy", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("without key: status = %d, want 401", rec.Code)
	}
	if rec.Body.String() == "did-deploy" {
		t.Fatal("action handler ran despite missing API key")
	}

	// Correct API key -> allowed.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/action/deploy", nil)
	req.Header.Set("X-API-Key", "secret")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("with key: status = %d, want 200", rec.Code)
	}
}
