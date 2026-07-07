package digitalocean

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// TestDeployInvokeRemoveAgainstMock exercises the DigitalOcean App Platform
// deploy → invoke → remove lifecycle through the provider's contract against a
// faithful App Platform REST + function-endpoint httptest double. DigitalOcean
// has no local emulator for its management API, so this override-driven mock is
// the reliable validation of the REST flow (app create, active-poll, invoke URL,
// delete).
func TestDeployInvokeRemoveAgainstMock(t *testing.T) {
	var (
		createHit bool
		pollHit   bool
		invokeHit bool
		deleteHit bool
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case r.Method == http.MethodPost && p == "/v2/apps":
			createHit = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"app":{"id":"app-123","live_url":"https://real.ondigitalocean.app"}}`))
		case r.Method == http.MethodGet && strings.HasPrefix(p, "/v2/apps/"):
			pollHit = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"app":{"phase":"ACTIVE"}}`))
		case r.Method == http.MethodDelete && strings.HasPrefix(p, "/v2/apps/"):
			deleteHit = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && strings.HasPrefix(p, "/app/"):
			invokeHit = true
			body, _ := io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true,"echo":` + jsonOrNull(body) + `}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	t.Setenv("DIGITALOCEAN_ENDPOINT_URL", srv.URL)
	t.Setenv("DIGITALOCEAN_ACCESS_TOKEN", "token")
	t.Setenv("DO_APP_REPO", "owner/repo")

	ctx := context.Background()
	stage := "dev"
	cfg := sdkprovider.Config{
		"service":  "svc",
		"provider": map[string]any{"name": "digitalocean-functions"},
		"functions": map[string]any{
			"hello": map[string]any{"handler": "index.main"},
		},
	}
	name := "svc-dev"

	root := t.TempDir()

	// --- Deploy: creates the app and polls until active ---
	dep, err := Runner{}.Deploy(ctx, cfg, stage, root)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if !createHit {
		t.Errorf("Deploy did not create the app")
	}
	if !pollHit {
		t.Errorf("Deploy did not poll for active phase")
	}
	if got := dep.Outputs["app_id"]; got != "app-123" {
		t.Errorf("app_id = %q, want %q", got, "app-123")
	}
	wantURL := srv.URL + "/app/" + name
	if got := dep.Outputs["url"]; got != wantURL {
		t.Errorf("url = %q, want %q", got, wantURL)
	}

	// --- Invoke: hits the emitted app URL and returns the real payload ---
	res, err := Invoker{}.Invoke(ctx, cfg, stage, "hello", []byte(`{"n":1}`), sdkprovider.ReceiptView{Outputs: dep.Outputs})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !invokeHit {
		t.Errorf("Invoke did not reach the app endpoint")
	}
	if !strings.Contains(res.Output, `"ok":true`) || !strings.Contains(res.Output, `"n":1`) {
		t.Errorf("Invoke did not return the real app payload; got %q", res.Output)
	}

	// --- Remove: deletes the app ---
	rem, err := Remover{}.Remove(ctx, cfg, stage, root, sdkprovider.ReceiptView{Outputs: dep.Outputs})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !rem.Removed {
		t.Errorf("Remove did not report Removed")
	}
	if !deleteHit {
		t.Errorf("Remove did not issue an app delete")
	}
}

// jsonOrNull returns the trimmed body as a JSON value, or null when empty.
func jsonOrNull(b []byte) string {
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "null"
	}
	return s
}
