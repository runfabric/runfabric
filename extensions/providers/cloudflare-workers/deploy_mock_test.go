package cloudflare

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

// TestDeployInvokeRemoveAgainstMock exercises the Cloudflare Workers deploy →
// invoke → remove lifecycle through the provider's contract against a faithful
// Cloudflare-API + worker httptest double. Cloudflare has no local emulator for
// its management API, so this override-driven mock is the reliable validation of
// the REST flow (script upload, invoke URL, delete).
func TestDeployInvokeRemoveAgainstMock(t *testing.T) {
	var (
		scriptBytes int
		putHit      bool
		invokeHit   bool
		deleteHit   bool
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case r.Method == http.MethodPut && strings.Contains(p, "/workers/scripts/"):
			putHit = true
			b, _ := io.ReadAll(r.Body)
			scriptBytes = len(b)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true,"result":{"id":"svc-dev"}}`))
		case r.Method == http.MethodDelete && strings.Contains(p, "/workers/scripts/"):
			deleteHit = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		case r.Method == http.MethodPost && strings.Contains(p, "/app/"):
			invokeHit = true
			body, _ := io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true,"echo":` + jsonOrNull(body) + `}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		}
	}))
	defer srv.Close()

	t.Setenv("CLOUDFLARE_ENDPOINT_URL", srv.URL)
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "acct-123")
	t.Setenv("CLOUDFLARE_API_TOKEN", "token")

	ctx := context.Background()
	stage := "dev"
	cfg := sdkprovider.Config{
		"service":  "svc",
		"provider": map[string]any{"name": "cloudflare-workers"},
		"functions": map[string]any{
			"hello": map[string]any{"handler": "worker.fetch"},
		},
	}
	name := "svc-dev"

	root := t.TempDir()
	if err := os.WriteFile(root+"/worker.js", []byte("export default { async fetch(req){ return new Response('ok'); } }\n"), 0o644); err != nil {
		t.Fatalf("write worker: %v", err)
	}

	// --- Deploy: uploads the worker script ---
	dep, err := Runner{}.Deploy(ctx, cfg, stage, root)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if !putHit {
		t.Errorf("Deploy did not upload the worker script")
	}
	if scriptBytes == 0 {
		t.Errorf("worker upload was empty")
	}
	wantURL := srv.URL + "/app/" + name
	if got := dep.Outputs["url"]; got != wantURL {
		t.Errorf("url = %q, want %q", got, wantURL)
	}

	// --- Invoke: hits the emitted worker URL and returns the real payload ---
	res, err := Invoker{}.Invoke(ctx, cfg, stage, "hello", []byte(`{"n":1}`), sdkprovider.ReceiptView{Outputs: dep.Outputs})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !invokeHit {
		t.Errorf("Invoke did not reach the worker endpoint")
	}
	if !strings.Contains(res.Output, `"ok":true`) || !strings.Contains(res.Output, `"n":1`) {
		t.Errorf("Invoke did not return the real worker payload; got %q", res.Output)
	}

	// --- Remove: deletes the worker script ---
	rem, err := Remover{}.Remove(ctx, cfg, stage, root, sdkprovider.ReceiptView{Metadata: dep.Metadata})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !rem.Removed {
		t.Errorf("Remove did not report Removed")
	}
	if !deleteHit {
		t.Errorf("Remove did not issue a worker delete")
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
