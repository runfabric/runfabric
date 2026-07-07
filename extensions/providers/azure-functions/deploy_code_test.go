package azure

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

// TestDeployPushesCodeAndInvokeReturnsPayload verifies the real Azure code-push
// path end-to-end against a faithful ARM + Kudu + function double: Deploy creates
// the resource group and site (ARM), zip-deploys the handler (Kudu), and emits a
// per-function invoke URL; Invoke then POSTs to that URL and gets the real
// payload back. floci-az cannot exercise this (it lacks Kudu and has no arm64
// runtime image), so this httptest double is the reliable validation of the
// mechanism; the floci-az lane covers the ARM control-plane lifecycle instead.
func TestDeployPushesCodeAndInvokeReturnsPayload(t *testing.T) {
	var (
		zipBytesReceived int
		zipDeployHit     bool
		invokeHit        bool
		siteDeleted      bool
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		// Kudu zip deploy.
		case strings.HasSuffix(p, "/api/zipdeploy"):
			zipDeployHit = true
			b, _ := io.ReadAll(r.Body)
			zipBytesReceived = len(b)
			w.WriteHeader(http.StatusOK)

		// Function invocation: /app/<app>/api/<fn>.
		case strings.Contains(p, "/app/") && strings.Contains(p, "/api/"):
			invokeHit = true
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true,"echo":` + string(quoteJSON(body)) + `}`))

		// ARM site resource (PUT create / GET poll / DELETE remove).
		case strings.Contains(p, "/providers/Microsoft.Web/sites/"):
			if r.Method == http.MethodDelete {
				siteDeleted = true
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"properties":{"state":"Running"}}`))

		// ARM resource group PUT.
		case strings.Contains(strings.ToLower(p), "/resourcegroups/"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"properties":{"provisioningState":"Succeeded"}}`))

		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	t.Setenv("AZURE_ENDPOINT_URL", srv.URL)
	t.Setenv("AZURE_ACCESS_TOKEN", "test")
	t.Setenv("AZURE_SUBSCRIPTION_ID", "00000000-0000-0000-0000-000000000000")
	t.Setenv("AZURE_RESOURCE_GROUP", "")

	ctx := context.Background()
	stage := "dev"
	cfg := sdkprovider.Config{
		"service": "svc",
		"provider": map[string]any{
			"name":    "azure-functions",
			"runtime": "node",
			"region":  "westus2",
		},
		"functions": map[string]any{
			"hello": map[string]any{"handler": "index.handler"},
		},
	}
	appName := "svc-dev"

	root := t.TempDir()
	if err := os.WriteFile(root+"/index.js", []byte("module.exports = async (c, req) => { c.res = { body: req.body }; };\n"), 0o644); err != nil {
		t.Fatalf("write handler: %v", err)
	}

	// --- Deploy: ARM app + real Kudu zip deploy ---
	dep, err := Runner{}.Deploy(ctx, cfg, stage, root)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if !zipDeployHit {
		t.Errorf("Deploy did not push code via Kudu zip deploy")
	}
	if zipBytesReceived == 0 {
		t.Errorf("Kudu zip deploy received an empty archive")
	}
	if got := dep.Outputs["code_deploy"]; got != "deployed" {
		t.Errorf("code_deploy = %q, want \"deployed\"", got)
	}
	wantURL := srv.URL + "/app/" + appName + "/api/hello"
	if got := dep.Outputs["url_hello"]; got != wantURL {
		t.Errorf("url_hello = %q, want %q", got, wantURL)
	}

	// --- Invoke: hits the emitted per-function URL and returns the real payload ---
	res, err := Invoker{}.Invoke(ctx, cfg, stage, "hello", []byte(`{"n":1}`), sdkprovider.ReceiptView{Outputs: dep.Outputs})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !invokeHit {
		t.Errorf("Invoke did not reach the function endpoint")
	}
	if !strings.Contains(res.Output, `"ok":true`) || !strings.Contains(res.Output, `"n":1`) {
		t.Errorf("Invoke did not return the real function payload; got %q", res.Output)
	}

	// --- Remove: deletes the ARM site ---
	rem, err := Remover{}.Remove(ctx, cfg, stage, root, sdkprovider.ReceiptView{Outputs: dep.Outputs})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !rem.Removed {
		t.Errorf("Remove did not report Removed")
	}
	if !siteDeleted {
		t.Errorf("Remove did not issue an ARM site delete")
	}
}

// quoteJSON returns a minimal JSON string literal for the given bytes, treating
// an empty body as null.
func quoteJSON(b []byte) []byte {
	s := strings.TrimSpace(string(b))
	if s == "" {
		return []byte("null")
	}
	return []byte(s)
}
