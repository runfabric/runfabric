package gcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

func TestRedirectToTunnel_ConditionalMutationAndRestore(t *testing.T) {
	var (
		patchApplied   bool
		restorePatched bool
	)

	service := "demo"
	stage := "dev"
	fnName := service + "-" + stage + "-api"
	project := "p1"
	region := "us-central1"
	resource := "projects/" + project + "/locations/" + region + "/functions/" + fnName

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/"+resource:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"serviceConfig":{"environmentVariables":{"A":"1"}}}`))
			return
		case r.Method == http.MethodPatch && r.URL.Path == "/v2/"+resource:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode patch body: %v", err)
			}
			sc, _ := body["serviceConfig"].(map[string]any)
			env, _ := sc["environmentVariables"].(map[string]any)
			if _, ok := env["RUNFABRIC_TUNNEL_URL"]; ok {
				patchApplied = true
			}
			if _, ok := env["RUNFABRIC_TUNNEL_URL"]; !ok {
				restorePatched = true
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	oldAPI := gcpFunctionsAPI
	gcpFunctionsAPI = ts.URL + "/v2"
	defer func() { gcpFunctionsAPI = oldAPI }()

	oldClient := apiutil.DefaultClient
	apiutil.DefaultClient = ts.Client()
	defer func() { apiutil.DefaultClient = oldClient }()

	t.Setenv("GCP_ACCESS_TOKEN", "token")
	t.Setenv("GCP_PROJECT", project)
	t.Setenv("GCP_PROJECT_ID", "")
	t.Setenv("GCP_DEV_STREAM_GATEWAY_SET_URL", "")
	t.Setenv("GCP_DEV_STREAM_GATEWAY_RESTORE_URL", "")

	cfg := &config.Config{
		Service: service,
		Provider: config.ProviderConfig{
			Name:   "gcp-functions",
			Region: region,
		},
		Functions: map[string]config.FunctionConfig{
			"api": {Handler: "index.handler"},
		},
	}

	state, err := RedirectToTunnel(context.Background(), cfg, stage, "https://abc.ngrok.io")
	if err != nil {
		t.Fatalf("redirect failed: %v", err)
	}
	if state == nil || !state.Applied {
		t.Fatal("expected applied state")
	}
	if state.EffectiveMode != "conditional-mutation" {
		t.Fatalf("expected conditional-mutation mode, got %q", state.EffectiveMode)
	}
	if !strings.Contains(state.StatusMessage, "not a full route rewrite") {
		t.Fatalf("unexpected status: %q", state.StatusMessage)
	}
	if !patchApplied {
		t.Fatal("expected patch with tunnel env vars")
	}

	if err := state.Restore(context.Background(), region); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if !restorePatched {
		t.Fatal("expected restore patch call")
	}
}

func TestRedirectToTunnel_GatewayHookRouteRewrite(t *testing.T) {
	var (
		gatewaySetCalled     bool
		gatewayRestoreCalled bool
	)

	service := "demo"
	stage := "dev"
	project := "p1"
	region := "us-central1"
	fnName := service + "-" + stage + "-api"

	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/set":
			gatewaySetCalled = true
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode set body: %v", err)
			}
			if body["function"] != fnName || body["tunnelUrl"] != "https://abc.ngrok.io" {
				t.Fatalf("unexpected set payload: %#v", body)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/restore":
			gatewayRestoreCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Fatalf("unexpected gateway path: %s", r.URL.Path)
		}
	}))
	defer gateway.Close()

	oldClient := apiutil.DefaultClient
	apiutil.DefaultClient = gateway.Client()
	defer func() { apiutil.DefaultClient = oldClient }()

	t.Setenv("GCP_ACCESS_TOKEN", "token")
	t.Setenv("GCP_PROJECT", project)
	t.Setenv("GCP_PROJECT_ID", "")
	t.Setenv("GCP_DEV_STREAM_GATEWAY_SET_URL", gateway.URL+"/set")
	t.Setenv("GCP_DEV_STREAM_GATEWAY_RESTORE_URL", gateway.URL+"/restore")

	cfg := &config.Config{
		Service: service,
		Provider: config.ProviderConfig{
			Name:   "gcp-functions",
			Region: region,
		},
		Functions: map[string]config.FunctionConfig{
			"api": {Handler: "index.handler"},
		},
	}

	state, err := RedirectToTunnel(context.Background(), cfg, stage, "https://abc.ngrok.io")
	if err != nil {
		t.Fatalf("redirect failed: %v", err)
	}
	if state == nil || !state.Applied || !state.GatewayApplied {
		t.Fatal("expected applied gateway route rewrite state")
	}
	if state.EffectiveMode != "route-rewrite" {
		t.Fatalf("expected route-rewrite mode, got %q", state.EffectiveMode)
	}
	if !strings.Contains(state.StatusMessage, "gateway-owned route rewrite applied") {
		t.Fatalf("unexpected status: %q", state.StatusMessage)
	}
	if !gatewaySetCalled {
		t.Fatal("expected gateway set hook call")
	}

	if err := state.Restore(context.Background(), region); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if !gatewayRestoreCalled {
		t.Fatal("expected gateway restore hook call")
	}
}

func TestRedirectToTunnel_LookupFailureFallsBack(t *testing.T) {
	service := "demo"
	stage := "dev"
	fnName := service + "-" + stage + "-api"
	project := "p1"
	region := "us-central1"
	resource := "projects/" + project + "/locations/" + region + "/functions/" + fnName

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v2/"+resource {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`not found`))
			return
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	defer ts.Close()

	oldAPI := gcpFunctionsAPI
	gcpFunctionsAPI = ts.URL + "/v2"
	defer func() { gcpFunctionsAPI = oldAPI }()

	oldClient := apiutil.DefaultClient
	apiutil.DefaultClient = ts.Client()
	defer func() { apiutil.DefaultClient = oldClient }()

	t.Setenv("GCP_ACCESS_TOKEN", "token")
	t.Setenv("GCP_PROJECT", project)
	t.Setenv("GCP_PROJECT_ID", "")
	t.Setenv("GCP_DEV_STREAM_GATEWAY_SET_URL", "")
	t.Setenv("GCP_DEV_STREAM_GATEWAY_RESTORE_URL", "")

	cfg := &config.Config{
		Service: service,
		Provider: config.ProviderConfig{
			Name:   "gcp-functions",
			Region: region,
		},
		Functions: map[string]config.FunctionConfig{
			"api": {Handler: "index.handler"},
		},
	}

	state, err := RedirectToTunnel(context.Background(), cfg, stage, "https://abc.ngrok.io")
	if err != nil {
		t.Fatalf("redirect failed: %v", err)
	}
	if state == nil {
		t.Fatal("expected state")
	}
	if state.Applied {
		t.Fatal("expected no applied mutation on lookup failure")
	}
	if state.EffectiveMode != "lifecycle-only" {
		t.Fatalf("expected lifecycle-only fallback, got %q", state.EffectiveMode)
	}
	if !strings.Contains(state.StatusMessage, "lookup returned") {
		t.Fatalf("unexpected status message: %q", state.StatusMessage)
	}
}
