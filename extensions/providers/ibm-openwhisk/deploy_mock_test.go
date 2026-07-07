package ibm

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

// TestDeployInvokeRemoveAgainstMock exercises the IBM OpenWhisk deploy → invoke
// → remove lifecycle through the provider's contract against a faithful
// OpenWhisk-API httptest double. OpenWhisk serves control- and data-plane calls
// from a single API host, so this IBM_OPENWHISK_ENDPOINT_URL override-driven
// mock is the reliable validation of the REST flow (action PUT, blocking
// invoke, action DELETE).
func TestDeployInvokeRemoveAgainstMock(t *testing.T) {
	var (
		putHit    bool
		invokeHit bool
		deleteHit bool
		codeBytes int
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case r.Method == http.MethodPut && strings.Contains(p, "/actions/"):
			putHit = true
			b, _ := io.ReadAll(r.Body)
			codeBytes = len(b)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"svc_dev_hello","version":"0.0.1"}`))
		case r.Method == http.MethodDelete && strings.Contains(p, "/actions/"):
			deleteHit = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"svc_dev_hello"}`))
		case r.Method == http.MethodPost && strings.Contains(p, "/actions/"):
			invokeHit = true
			body, _ := io.ReadAll(r.Body)
			// Echo the invocation input back as the activation result.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"response":{"success":true,"result":` + jsonOrNull(body) + `}}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	t.Setenv("IBM_OPENWHISK_ENDPOINT_URL", srv.URL)
	t.Setenv("IBM_OPENWHISK_AUTH", "user:secret")
	t.Setenv("IBM_OPENWHISK_NAMESPACE", "_")

	ctx := context.Background()
	stage := "dev"
	cfg := sdkprovider.Config{
		"service":  "svc",
		"provider": map[string]any{"name": "ibm-openwhisk"},
		"functions": map[string]any{
			"hello": map[string]any{"handler": "index.handler"},
		},
	}

	root := t.TempDir()
	if err := os.WriteFile(root+"/index.js", []byte("function main(args){ return {ok:true}; }\nexports.main = main;\n"), 0o644); err != nil {
		t.Fatalf("write action code: %v", err)
	}

	// --- Deploy: PUTs the action code ---
	dep, err := Runner{}.Deploy(ctx, cfg, stage, root)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if !putHit {
		t.Errorf("Deploy did not PUT the action")
	}
	if codeBytes == 0 {
		t.Errorf("action create body was empty")
	}
	wantURL := srv.URL + "/api/v1/namespaces/_/actions/svc_dev_hello"
	if got := dep.Outputs["action_hello"]; got != wantURL {
		t.Errorf("action_hello = %q, want %q", got, wantURL)
	}

	// --- Invoke: blocking POST returns the real activation payload ---
	res, err := Invoker{}.Invoke(ctx, cfg, stage, "hello", []byte(`{"n":1}`), nil)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !invokeHit {
		t.Errorf("Invoke did not reach the action endpoint")
	}
	if !strings.Contains(res.Output, `"success":true`) || !strings.Contains(res.Output, `"n":1`) {
		t.Errorf("Invoke did not return the real activation payload; got %q", res.Output)
	}

	// --- Remove: DELETEs the action ---
	rem, err := Remover{}.Remove(ctx, cfg, stage, root, nil)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !rem.Removed {
		t.Errorf("Remove did not report Removed")
	}
	if !deleteHit {
		t.Errorf("Remove did not issue an action delete")
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
