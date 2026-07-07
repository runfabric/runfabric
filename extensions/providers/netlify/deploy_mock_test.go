package netlify

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

// TestDeployInvokeRemoveAgainstMock exercises the Netlify deploy → invoke →
// remove lifecycle through the provider's contract against a faithful Netlify
// REST API + site httptest double. Netlify has no local emulator for its
// management API, so this override-driven mock is the reliable validation of the
// REST flow (create site, upload deploy zip, poll ready, invoke URL, delete).
func TestDeployInvokeRemoveAgainstMock(t *testing.T) {
	var (
		siteCreated bool
		zipBytes    int
		deployHit   bool
		invokeHit   bool
		deleteHit   bool
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		// Create deploy: POST /sites/<id>/deploys (multipart zip upload).
		case r.Method == http.MethodPost && strings.HasSuffix(p, "/deploys"):
			deployHit = true
			b, _ := io.ReadAll(r.Body)
			zipBytes = len(b)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"dep-1","deploy_ssl_url":"https://svc-dev.netlify.app"}`))
		// Poll deploy state: GET /deploys/<id>.
		case r.Method == http.MethodGet && strings.Contains(p, "/deploys/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"state":"ready"}`))
		// Create site: POST /sites.
		case r.Method == http.MethodPost && strings.HasSuffix(p, "/sites"):
			siteCreated = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"site-123"}`))
		// Delete site: DELETE /sites/<id>.
		case r.Method == http.MethodDelete && strings.Contains(p, "/sites/"):
			deleteHit = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		// Invocation: POST /app/<name>.
		case r.Method == http.MethodPost && strings.Contains(p, "/app/"):
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

	t.Setenv("NETLIFY_ENDPOINT_URL", srv.URL)
	t.Setenv("NETLIFY_AUTH_TOKEN", "token")
	t.Setenv("NETLIFY_SITE_ID", "")

	ctx := context.Background()
	stage := "dev"
	cfg := sdkprovider.Config{
		"service":  "svc",
		"provider": map[string]any{"name": "netlify"},
		"functions": map[string]any{
			"hello": map[string]any{"handler": "index.handler"},
		},
	}
	name := "svc-dev"

	root := t.TempDir()
	if err := os.WriteFile(root+"/index.js", []byte("exports.handler = async (e) => ({ statusCode: 200, body: e.body });\n"), 0o644); err != nil {
		t.Fatalf("write handler: %v", err)
	}

	// --- Deploy: creates the site and uploads the deploy zip ---
	dep, err := Runner{}.Deploy(ctx, cfg, stage, root)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if !siteCreated {
		t.Errorf("Deploy did not create the site")
	}
	if !deployHit {
		t.Errorf("Deploy did not upload the deploy zip")
	}
	if zipBytes == 0 {
		t.Errorf("deploy upload was empty")
	}
	wantURL := srv.URL + "/app/" + name
	if got := dep.Outputs["url"]; got != wantURL {
		t.Errorf("url = %q, want %q", got, wantURL)
	}

	// --- Invoke: hits the emitted site URL and returns the real payload ---
	res, err := Invoker{}.Invoke(ctx, cfg, stage, "hello", []byte(`{"n":1}`), sdkprovider.ReceiptView{Outputs: dep.Outputs})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !invokeHit {
		t.Errorf("Invoke did not reach the site endpoint")
	}
	if !strings.Contains(res.Output, `"ok":true`) || !strings.Contains(res.Output, `"n":1`) {
		t.Errorf("Invoke did not return the real site payload; got %q", res.Output)
	}

	// --- Remove: deletes the site ---
	rem, err := Remover{}.Remove(ctx, cfg, stage, root, sdkprovider.ReceiptView{Outputs: dep.Outputs, Metadata: dep.Metadata})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !rem.Removed {
		t.Errorf("Remove did not report Removed")
	}
	if !deleteHit {
		t.Errorf("Remove did not issue a site delete")
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
