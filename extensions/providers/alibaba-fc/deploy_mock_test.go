package alibaba

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

// TestDeployInvokeRemoveAgainstMock exercises the Alibaba FC deploy → invoke →
// remove lifecycle through the provider's contract against a faithful FC-OpenAPI
// + function-endpoint httptest double. Alibaba has no local emulator for its
// signed OpenAPI, so this override-driven mock is the reliable validation of the
// REST flow (service/function create with code upload, invoke URL, delete).
func TestDeployInvokeRemoveAgainstMock(t *testing.T) {
	var (
		codeBytes    int
		createFnHit  bool
		invokeHit    bool
		deleteFnHit  bool
		deleteSvcHit bool
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		// Function invocation: /<version>/proxy/<service>/<function>/.
		case r.Method == http.MethodPost && strings.Contains(p, "/proxy/"):
			invokeHit = true
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true,"echo":` + jsonOrNull(body) + `}`))

		// CreateFunction (uploads the code zip).
		case r.Method == http.MethodPost && strings.HasSuffix(p, "/functions"):
			createFnHit = true
			b, _ := io.ReadAll(r.Body)
			codeBytes = len(b)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"functionName":"fn"}`))

		// GetFunction readiness poll.
		case r.Method == http.MethodGet && strings.Contains(p, "/functions/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"functionName":"fn"}`))

		// DeleteFunction.
		case r.Method == http.MethodDelete && strings.Contains(p, "/functions/"):
			deleteFnHit = true
			w.WriteHeader(http.StatusOK)

		// DeleteService.
		case r.Method == http.MethodDelete && strings.Contains(p, "/services/"):
			deleteSvcHit = true
			w.WriteHeader(http.StatusOK)

		// CreateService, CreateTrigger, etc.
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	t.Setenv("ALIBABA_ENDPOINT_URL", srv.URL)
	t.Setenv("ALIBABA_ACCESS_KEY_ID", "akid")
	t.Setenv("ALIBABA_ACCESS_KEY_SECRET", "secret")
	t.Setenv("ALIBABA_FC_ACCOUNT_ID", "acct-123")

	ctx := context.Background()
	stage := "dev"
	cfg := sdkprovider.Config{
		"service":  "svc",
		"provider": map[string]any{"name": "alibaba-fc", "region": "cn-hangzhou"},
		"functions": map[string]any{
			"hello": map[string]any{"handler": "index.handler"},
		},
	}
	serviceName := "svc-" + stage
	funcName := "svc-" + stage + "-hello"

	root := t.TempDir()
	if err := os.WriteFile(root+"/index.js", []byte("exports.handler = (req, resp) => resp.send(req.body);\n"), 0o644); err != nil {
		t.Fatalf("write handler: %v", err)
	}

	// --- Deploy: creates the service/function and uploads the code zip ---
	dep, err := Runner{}.Deploy(ctx, cfg, stage, root)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if !createFnHit {
		t.Errorf("Deploy did not create the function")
	}
	if codeBytes == 0 {
		t.Errorf("function code upload was empty")
	}
	wantURL := srv.URL + "/" + fcAPIVersion + "/proxy/" + serviceName + "/" + funcName + "/"
	if got := dep.Outputs["url"]; got != wantURL {
		t.Errorf("url = %q, want %q", got, wantURL)
	}

	// --- Invoke: hits the emitted function URL and returns the real payload ---
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

	// --- Remove: deletes the function and service ---
	rem, err := Remover{}.Remove(ctx, cfg, stage, root, sdkprovider.ReceiptView{Outputs: dep.Outputs, Metadata: dep.Metadata})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !rem.Removed {
		t.Errorf("Remove did not report Removed")
	}
	if !deleteFnHit {
		t.Errorf("Remove did not issue a function delete")
	}
	if !deleteSvcHit {
		t.Errorf("Remove did not issue a service delete")
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
