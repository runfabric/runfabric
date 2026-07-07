package fly

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// TestDeployInvokeRemoveAgainstMock exercises the Fly Machines deploy → invoke →
// remove lifecycle through the provider's contract against a faithful Machines
// REST API + app-endpoint httptest double. Fly has no local emulator for its
// Machines API, so this override-driven mock is the reliable validation of the
// REST flow (create app, launch machine, wait for started, invoke URL, delete).
func TestDeployInvokeRemoveAgainstMock(t *testing.T) {
	var (
		appHit     bool
		machineHit bool
		stateHit   bool
		invokeHit  bool
		deleteHit  bool
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(p, "/apps"):
			appHit = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"name":"svc-dev"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(p, "/machines"):
			machineHit = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"machine-123"}`))
		case r.Method == http.MethodGet && strings.Contains(p, "/machines/"):
			stateHit = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"state":"started"}`))
		case r.Method == http.MethodDelete && strings.Contains(p, "/apps/"):
			deleteHit = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodPost && strings.Contains(p, "/app/"):
			invokeHit = true
			body, _ := io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true,"echo":` + jsonOrNull(body) + `}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer srv.Close()

	t.Setenv("FLY_ENDPOINT_URL", srv.URL)
	t.Setenv("FLY_API_TOKEN", "token")
	t.Setenv("FLY_IMAGE", "registry.fly.io/svc:latest")

	ctx := context.Background()
	stage := "dev"
	cfg := sdkprovider.Config{
		"service":  "svc",
		"provider": map[string]any{"name": "fly-machines"},
		"functions": map[string]any{
			"hello": map[string]any{"handler": "main"},
		},
	}
	name := "svc-dev"
	root := t.TempDir()

	// --- Deploy: creates the app, launches a machine, waits for started ---
	dep, err := Runner{}.Deploy(ctx, cfg, stage, root)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if !appHit {
		t.Errorf("Deploy did not create the app")
	}
	if !machineHit {
		t.Errorf("Deploy did not launch a machine")
	}
	if !stateHit {
		t.Errorf("Deploy did not poll machine state")
	}
	wantURL := srv.URL + "/app/" + name
	if got := dep.Outputs["url"]; got != wantURL {
		t.Errorf("url = %q, want %q", got, wantURL)
	}
	if got := dep.Metadata["app"]; got != name {
		t.Errorf("app metadata = %q, want %q", got, name)
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
	rem, err := Remover{}.Remove(ctx, cfg, stage, root, sdkprovider.ReceiptView{Metadata: dep.Metadata})
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
