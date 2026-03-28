package external

import (
	"context"
	"testing"

	routercontracts "github.com/runfabric/runfabric/platform/core/contracts/router"
)

func TestExternalRouterAdapter_Sync(t *testing.T) {
	exe := buildStubPlugin(t)
	adapter := NewExternalRouterAdapter("stub-router", exe, routercontracts.PluginMeta{ID: "stub-router", Name: "Stub Router"})

	res, err := adapter.Sync(context.Background(), routercontracts.SyncRequest{
		Routing: &routercontracts.RoutingConfig{
			Contract: "runfabric.fabric.routing.v1",
			Service:  "svc",
			Stage:    "dev",
			Hostname: "svc.example.com",
			Strategy: "round-robin",
			TTL:      300,
			Endpoints: []routercontracts.RoutingEndpoint{
				{Name: "aws", URL: "https://aws.example.com"},
			},
		},
		ZoneID:    "zone-1",
		AccountID: "acct-1",
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil sync result")
	}
	if len(res.Actions) == 0 {
		t.Fatalf("expected sync actions, got %#v", res)
	}
	if res.Actions[0].Resource != "dns_record" {
		t.Fatalf("expected dns_record action, got %#v", res.Actions[0])
	}
}
