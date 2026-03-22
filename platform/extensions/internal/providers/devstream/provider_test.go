package devstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

func TestRedirectToTunnel_GatewayHooksApplyAndRestore(t *testing.T) {
	setCalled := false
	restoreCalled := false
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/set":
			setCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/restore":
			restoreCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer gateway.Close()

	oldClient := apiutil.DefaultClient
	apiutil.DefaultClient = gateway.Client()
	defer func() { apiutil.DefaultClient = oldClient }()

	t.Setenv("RUNFABRIC_DEV_STREAM_AZURE_FUNCTIONS_SET_URL", gateway.URL+"/set")
	t.Setenv("RUNFABRIC_DEV_STREAM_AZURE_FUNCTIONS_RESTORE_URL", gateway.URL+"/restore")
	t.Setenv("RUNFABRIC_DEV_STREAM_AZURE_FUNCTIONS_TOKEN", "")

	cfg := &config.Config{Service: "svc"}
	state, err := RedirectToTunnel("azure-functions", cfg, "dev", "https://abc.ngrok.io")
	if err != nil {
		t.Fatalf("redirect failed: %v", err)
	}
	if state == nil {
		t.Fatal("expected state")
	}
	if state.Mode != "route-rewrite" {
		t.Fatalf("expected route-rewrite mode, got %q", state.Mode)
	}
	if !state.GatewayApplied {
		t.Fatal("expected gateway applied")
	}
	if !setCalled {
		t.Fatal("expected set hook call")
	}

	if err := state.Restore(context.Background()); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if !restoreCalled {
		t.Fatal("expected restore hook call")
	}
}

func TestRedirectToTunnel_MissingGatewayHooksFallsBack(t *testing.T) {
	t.Setenv("RUNFABRIC_DEV_STREAM_AZURE_FUNCTIONS_SET_URL", "")
	t.Setenv("RUNFABRIC_DEV_STREAM_AZURE_FUNCTIONS_RESTORE_URL", "")

	cfg := &config.Config{Service: "svc"}
	state, err := RedirectToTunnel("azure-functions", cfg, "dev", "https://abc.ngrok.io")
	if err != nil {
		t.Fatalf("redirect failed: %v", err)
	}
	if state == nil {
		t.Fatal("expected state")
	}
	if state.Mode != "lifecycle-only" {
		t.Fatalf("expected lifecycle-only mode, got %q", state.Mode)
	}
	if len(state.MissingPrereqs) != 2 {
		t.Fatalf("expected 2 missing prereqs, got %d", len(state.MissingPrereqs))
	}
}
