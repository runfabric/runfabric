package resolution

import (
	"context"
	"testing"

	routercontracts "github.com/runfabric/runfabric/platform/core/contracts/router"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
)

func TestResolveRuntime_NormalizesVersionedRuntimeIDs(t *testing.T) {
	b, err := New(Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}

	got, err := b.ResolveRuntime("nodejs20.x")
	if err != nil {
		t.Fatalf("resolve nodejs20.x: %v", err)
	}
	if got.ID != "nodejs" {
		t.Fatalf("runtime id = %q, want nodejs", got.ID)
	}

	got, err = b.ResolveRuntime("python3.11")
	if err != nil {
		t.Fatalf("resolve python3.11: %v", err)
	}
	if got.ID != "python" {
		t.Fatalf("runtime id = %q, want python", got.ID)
	}

	rt, err := b.ResolveRuntimePlugin("nodejs20.x")
	if err != nil {
		t.Fatalf("resolve runtime plugin nodejs20.x: %v", err)
	}
	if rt.Meta().ID != "nodejs" {
		t.Fatalf("runtime plugin id = %q, want nodejs", rt.Meta().ID)
	}
}

func TestResolveRuntime_UnknownRuntimeErrors(t *testing.T) {
	b, err := New(Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}

	if _, err := b.ResolveRuntime("unknown-runtime"); err == nil {
		t.Fatal("expected unknown runtime to return error")
	}
}

func TestResolveSimulator_BuiltinLocal(t *testing.T) {
	b, err := New(Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}
	sim, err := b.ResolveSimulator("local")
	if err != nil {
		t.Fatalf("resolve local simulator: %v", err)
	}
	if sim.Meta().ID != "local" {
		t.Fatalf("simulator id = %q, want local", sim.Meta().ID)
	}
}

func TestResolveRouter_Builtin(t *testing.T) {
	b, err := New(Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}
	routers := b.PluginRegistry().List(manifests.KindRouter)
	if len(routers) == 0 {
		t.Fatal("expected at least one built-in router plugin")
	}
	wantID := routers[0].ID
	router, err := b.ResolveRouter(wantID)
	if err != nil {
		t.Fatalf("resolve router %q: %v", wantID, err)
	}
	if router.Meta().ID != wantID {
		t.Fatalf("router id = %q, want %q", router.Meta().ID, wantID)
	}
}

func TestResolveRouter_External(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNFABRIC_HOME", home)
	writeExternalRouter(t, home, "external-router", "0.1.0")

	b, err := New(Options{IncludeExternal: true})
	if err != nil {
		t.Fatalf("new boundary with external plugins: %v", err)
	}
	router, err := b.ResolveRouter("external-router")
	if err != nil {
		t.Fatalf("resolve external router: %v", err)
	}
	if router.Meta().ID != "external-router" {
		t.Fatalf("router id = %q, want external-router", router.Meta().ID)
	}
}

func TestSyncRouter_UnknownRouterReturnsError(t *testing.T) {
	b, err := New(Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}
	_, err = b.SyncRouter(context.Background(), "missing-router", routercontracts.SyncRequest{})
	if err == nil {
		t.Fatal("expected sync router error for unknown router")
	}
}

func TestSyncRouter_ExternalDispatchErrorBubblesUp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNFABRIC_HOME", home)
	writeExternalRouter(t, home, "external-router", "0.1.0")

	b, err := New(Options{IncludeExternal: true})
	if err != nil {
		t.Fatalf("new boundary with external plugins: %v", err)
	}
	_, err = b.SyncRouter(context.Background(), "external-router", routercontracts.SyncRequest{
		Routing: &routercontracts.RoutingConfig{
			Contract: "runfabric.fabric.routing.v1",
			Service:  "svc",
			Stage:    "dev",
			Hostname: "svc.example.com",
			Strategy: "round-robin",
			Endpoints: []routercontracts.RoutingEndpoint{
				{Name: "aws", URL: "https://aws.example.com", Weight: 100},
			},
		},
		DryRun: true,
	})
	if err == nil {
		t.Fatal("expected external router dispatch error for non-protocol executable")
	}
}
