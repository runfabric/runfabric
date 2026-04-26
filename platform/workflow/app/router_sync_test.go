package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	routercontracts "github.com/runfabric/runfabric/platform/core/contracts/router"
	"github.com/runfabric/runfabric/platform/core/model/config"
	statecore "github.com/runfabric/runfabric/platform/core/state/core"
)

type syncOnlyExtensions struct {
	result *routercontracts.SyncResult
	err    error
}

func (s *syncOnlyExtensions) ResolveProvider(name string) (*ConnectorProvider, error) {
	return nil, providers.ErrProviderNotFound(name)
}

func (s *syncOnlyExtensions) EnsureSimulator(simulatorID string) error { return nil }

func (s *syncOnlyExtensions) BuildFunction(ctx context.Context, req RuntimeBuildRequest) (*providers.Artifact, error) {
	return nil, nil
}

func (s *syncOnlyExtensions) Simulate(ctx context.Context, simulatorID string, req SimulatorInvokeRequest) (*SimulatorInvokeResult, error) {
	return nil, nil
}

func (s *syncOnlyExtensions) SyncRouter(ctx context.Context, routerID string, req RouterSyncRequest) (*routercontracts.SyncResult, error) {
	return s.result, s.err
}

func (s *syncOnlyExtensions) RefreshExternal() error { return nil }

func TestRouterDNSSync_AppendsHistorySnapshot(t *testing.T) {
	root := t.TempDir()
	ctx := &AppContext{
		Config: &config.Config{
			Service:    "svc",
			Extensions: map[string]any{"routerPlugin": "mock-router"},
		},
		Extensions: &syncOnlyExtensions{
			result: &routercontracts.SyncResult{
				DryRun: false,
				Actions: []routercontracts.SyncAction{
					{Resource: "dns_record", Action: "update", Name: "svc.example.com"},
				},
			},
		},
		RootDir: root,
		Stage:   "dev",
	}
	routing := &RouterRoutingConfig{
		Contract: "runfabric.fabric.routing.v1",
		Service:  "svc",
		Stage:    "dev",
		Hostname: "svc.example.com",
		Strategy: "round-robin",
		TTL:      300,
		Endpoints: []RouterRoutingEndpoint{
			{Name: "aws", URL: "https://aws.example.com", Weight: 100},
		},
	}

	_, err := RouterDNSSync(ctx, routing, "zone-1", "acct-1", false, io.Discard)
	if err != nil {
		t.Fatalf("RouterDNSSync returned error: %v", err)
	}
	history, err := statecore.LoadRouterSyncHistory(root, "dev")
	if err != nil {
		t.Fatalf("load router sync history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected one router sync snapshot, got %d", len(history))
	}
	snapshot := history[0]
	if snapshot.PluginID != "mock-router" {
		t.Fatalf("unexpected plugin id: %q", snapshot.PluginID)
	}
	if snapshot.ZoneID != "zone-1" || snapshot.AccountID != "acct-1" {
		t.Fatalf("unexpected provider ids in snapshot: zone=%q account=%q", snapshot.ZoneID, snapshot.AccountID)
	}
	if len(snapshot.Actions) != 1 || snapshot.Actions[0].Action != "update" {
		t.Fatalf("unexpected snapshot actions: %#v", snapshot.Actions)
	}
	if snapshot.Summary.Update != 1 {
		t.Fatalf("expected summary update=1, got %#v", snapshot.Summary)
	}
}

func TestRouterDNSSyncWithOptions_PersistsBeforeAfterAndOperationMetadata(t *testing.T) {
	root := t.TempDir()
	ctx := &AppContext{
		Config: &config.Config{
			Service:    "svc",
			Extensions: map[string]any{"routerPlugin": "mock-router"},
		},
		Extensions: &syncOnlyExtensions{
			result: &routercontracts.SyncResult{
				DryRun: false,
				Actions: []routercontracts.SyncAction{
					{Resource: "dns_record", Action: "update", Name: "svc.example.com"},
					{Resource: "lb_pool", Action: "create", Name: "svc-dev-pool"},
				},
			},
		},
		RootDir: root,
		Stage:   "dev",
	}
	routing := &RouterRoutingConfig{
		Contract: "runfabric.fabric.routing.v1",
		Service:  "svc",
		Stage:    "dev",
		Hostname: "svc.example.com",
		Strategy: "round-robin",
		TTL:      300,
		Endpoints: []RouterRoutingEndpoint{
			{Name: "aws", URL: "https://aws.example.com", Weight: 100},
		},
	}
	before := []routercontracts.SyncAction{
		{Resource: "dns_record", Action: "update", Name: "svc.example.com"},
	}
	_, err := RouterDNSSyncWithOptions(ctx, routing, "zone-1", "acct-1", false, io.Discard, RouterDNSSyncOptions{
		OperationID:   "op-123",
		Trigger:       "unit-test",
		BeforeActions: before,
		Events: []statecore.RouterSyncEvent{
			{Timestamp: "2026-03-28T00:00:00Z", Phase: "start", Message: "test start"},
		},
	})
	if err != nil {
		t.Fatalf("RouterDNSSyncWithOptions returned error: %v", err)
	}
	history, err := statecore.LoadRouterSyncHistory(root, "dev")
	if err != nil {
		t.Fatalf("load router sync history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected one router sync snapshot, got %d", len(history))
	}
	snapshot := history[0]
	if snapshot.Operation != "op-123" || snapshot.Trigger != "unit-test" {
		t.Fatalf("unexpected operation metadata: %#v", snapshot)
	}
	if len(snapshot.Before) != 1 || snapshot.Before[0].Action != "update" {
		t.Fatalf("unexpected before actions: %#v", snapshot.Before)
	}
	if snapshot.BeforeSum.Update != 1 {
		t.Fatalf("unexpected before summary: %#v", snapshot.BeforeSum)
	}
	if len(snapshot.After) != 2 || snapshot.AfterSum.Create != 1 || snapshot.AfterSum.Update != 1 {
		t.Fatalf("unexpected after snapshot state: after=%#v summary=%#v", snapshot.After, snapshot.AfterSum)
	}
	if len(snapshot.Events) < 2 {
		t.Fatalf("expected start + complete events, got %#v", snapshot.Events)
	}
}

func TestSelectedRouterPlugin_ResolvesByName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNFABRIC_HOME", home)

	pluginDir := filepath.Join(home, "plugins", "routers", "edge-router", "1.0.0")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "edge-router"), []byte("x"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	pluginYAML := []byte(`apiVersion: runfabric.io/plugin/v1
kind: router
id: edge-router
name: Edge Router
version: 1.0.0
executable: edge-router
`)
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), pluginYAML, 0o644); err != nil {
		t.Fatalf("write plugin.yaml: %v", err)
	}

	cfg := &config.Config{Extensions: map[string]any{"routerPlugin": "Edge Router"}}
	got := SelectedRouterPlugin(cfg)
	if got != "edge-router" {
		t.Fatalf("SelectedRouterPlugin=%q want edge-router", got)
	}
}
