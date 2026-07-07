package vercel

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// TestDeployInvokeRemoveAgainstMock exercises the Vercel deploy → invoke → remove
// lifecycle through the provider's contract against a faithful Vercel-REST-API +
// deployment httptest double. Vercel has no local emulator for its API, so this
// override-driven mock is the reliable validation of the REST flow (deployment
// create, ready poll, invoke URL, project delete).
func TestDeployInvokeRemoveAgainstMock(t *testing.T) {
	var (
		createHit bool
		fileCount int
		invokeHit bool
		deleteHit bool
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		// Deployment create.
		case r.Method == http.MethodPost && p == "/v13/deployments":
			createHit = true
			b, _ := io.ReadAll(r.Body)
			fileCount = len(b)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"dpl_123","url":"svc.vercel.app"}`))
		// Deployment ready poll.
		case r.Method == http.MethodGet && strings.HasPrefix(p, "/v13/deployments/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"readyState":"READY"}`))
		// Project delete.
		case r.Method == http.MethodDelete && strings.HasPrefix(p, "/v9/projects/"):
			deleteHit = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		// Deployment invocation: /app/<name>.
		case r.Method == http.MethodPost && strings.Contains(p, "/app/"):
			invokeHit = true
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true,"echo":` + jsonOrNull(body) + `}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	t.Setenv("VERCEL_ENDPOINT_URL", srv.URL)
	t.Setenv("VERCEL_TOKEN", "token")
	t.Setenv("VERCEL_TEAM_ID", "")

	ctx := context.Background()
	stage := "dev"
	cfg := sdkprovider.Config{
		"service":  "svc",
		"provider": map[string]any{"name": "vercel"},
		"functions": map[string]any{
			"hello": map[string]any{"handler": "index.handler"},
		},
	}
	name := "svc"

	root := t.TempDir()
	if err := os.WriteFile(root+"/index.js", []byte("module.exports = (req, res) => res.json(req.body);\n"), 0o644); err != nil {
		t.Fatalf("write handler: %v", err)
	}

	// --- Deploy: creates the deployment and emits the override-based invoke URL ---
	dep, err := Runner{}.Deploy(ctx, cfg, stage, root)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if !createHit {
		t.Errorf("Deploy did not create the deployment")
	}
	if fileCount == 0 {
		t.Errorf("deployment create payload was empty")
	}
	wantURL := srv.URL + "/app/" + name
	if got := dep.Outputs["url"]; got != wantURL {
		t.Errorf("url = %q, want %q", got, wantURL)
	}

	// --- Invoke: hits the emitted deployment URL and returns the real payload ---
	res, err := Invoker{}.Invoke(ctx, cfg, stage, "hello", []byte(`{"n":1}`), sdkprovider.ReceiptView{Outputs: dep.Outputs})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !invokeHit {
		t.Errorf("Invoke did not reach the deployment endpoint")
	}
	if !strings.Contains(res.Output, `"ok":true`) || !strings.Contains(res.Output, `"n":1`) {
		t.Errorf("Invoke did not return the real deployment payload; got %q", res.Output)
	}

	// --- Remove: deletes the project ---
	rem, err := Remover{}.Remove(ctx, cfg, stage, root, sdkprovider.ReceiptView{Metadata: dep.Metadata})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !rem.Removed {
		t.Errorf("Remove did not report Removed")
	}
	if !deleteHit {
		t.Errorf("Remove did not issue a project delete")
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
