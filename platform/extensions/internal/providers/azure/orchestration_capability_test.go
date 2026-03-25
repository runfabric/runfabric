package azure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func TestSyncOrchestrations_ManagesDurableAppSettings(t *testing.T) {
	var putBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/config/appsettings/list") && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{"properties": map[string]string{"EXISTING": "1"}})
		case strings.Contains(r.URL.Path, "/config/appsettings") && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Fatalf("decode put body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	oldClient := apiutil.DefaultClient
	apiutil.DefaultClient = ts.Client()
	defer func() { apiutil.DefaultClient = oldClient }()
	oldMgmtAPI := azureManagementAPI
	azureManagementAPI = ts.URL
	defer func() { azureManagementAPI = oldMgmtAPI }()

	t.Setenv("AZURE_ACCESS_TOKEN", "token")
	t.Setenv("AZURE_SUBSCRIPTION_ID", "sub-1")
	t.Setenv("AZURE_RESOURCE_GROUP", "rg-1")

	cfg := sdkprovider.Config{
		"service":  "svc",
		"provider": map[string]any{"name": "azure-functions"},
		"extensions": map[string]any{
			"azure-functions": map[string]any{
				"durableFunctions": []any{
					map[string]any{"name": "order-flow", "orchestrator": "OrderFlow", "taskHub": "orders-hub"},
				},
			},
		},
	}
	req := sdkprovider.OrchestrationSyncRequest{
		Config: cfg,
		Stage:  "dev",
		Root:   ".",
	}

	runner := Runner{}
	res, err := runner.SyncOrchestrations(context.Background(), req)
	if err != nil {
		t.Fatalf("sync orchestrations error: %v", err)
	}
	if res.Metadata["durable:order-flow:operation"] != "created" {
		t.Fatalf("expected created operation, got %q", res.Metadata["durable:order-flow:operation"])
	}

	props, _ := putBody["properties"].(map[string]any)
	if props["RUNFABRIC_DURABLE_ORDER_FLOW_MANAGED"] != "1" {
		t.Fatalf("expected managed flag in settings, got %#v", props)
	}
	if props["RUNFABRIC_DURABLE_ORDER_FLOW_ORCHESTRATOR"] != "OrderFlow" {
		t.Fatalf("expected orchestrator setting, got %#v", props)
	}
	if props["RUNFABRIC_DURABLE_ORDER_FLOW_TASK_HUB"] != "orders-hub" {
		t.Fatalf("expected task hub setting, got %#v", props)
	}
}

func TestRemoveOrchestrations_RemovesDurableAppSettings(t *testing.T) {
	var putBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/config/appsettings/list") && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{"properties": map[string]string{
				"RUNFABRIC_DURABLE_ORDER_FLOW_MANAGED":      "1",
				"RUNFABRIC_DURABLE_ORDER_FLOW_ORCHESTRATOR": "OrderFlow",
				"EXISTING": "1",
			}})
		case strings.Contains(r.URL.Path, "/config/appsettings") && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Fatalf("decode put body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	oldClient := apiutil.DefaultClient
	apiutil.DefaultClient = ts.Client()
	defer func() { apiutil.DefaultClient = oldClient }()
	oldMgmtAPI := azureManagementAPI
	azureManagementAPI = ts.URL
	defer func() { azureManagementAPI = oldMgmtAPI }()

	t.Setenv("AZURE_ACCESS_TOKEN", "token")
	t.Setenv("AZURE_SUBSCRIPTION_ID", "sub-1")
	t.Setenv("AZURE_RESOURCE_GROUP", "rg-1")

	cfg := sdkprovider.Config{
		"service":  "svc",
		"provider": map[string]any{"name": "azure-functions"},
		"extensions": map[string]any{
			"azure-functions": map[string]any{
				"durableFunctions": []any{
					map[string]any{"name": "order-flow", "orchestrator": "OrderFlow"},
				},
			},
		},
	}

	runner := Runner{}
	res, err := runner.RemoveOrchestrations(context.Background(), sdkprovider.OrchestrationRemoveRequest{Config: cfg, Stage: "dev", Root: "."})
	if err != nil {
		t.Fatalf("remove orchestrations error: %v", err)
	}
	if res.Metadata["durable:order-flow:operation"] != "deleted" {
		t.Fatalf("expected deleted operation, got %q", res.Metadata["durable:order-flow:operation"])
	}

	props, _ := putBody["properties"].(map[string]any)
	if _, ok := props["RUNFABRIC_DURABLE_ORDER_FLOW_MANAGED"]; ok {
		t.Fatalf("expected managed flag removed, got %#v", props)
	}
	if _, ok := props["RUNFABRIC_DURABLE_ORDER_FLOW_ORCHESTRATOR"]; ok {
		t.Fatalf("expected orchestrator removed, got %#v", props)
	}
	if props["EXISTING"] != "1" {
		t.Fatalf("expected unrelated settings preserved, got %#v", props)
	}
}
