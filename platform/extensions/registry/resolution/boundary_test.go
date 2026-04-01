package resolution

import (
	"context"
	"testing"

	routercontracts "github.com/runfabric/runfabric/platform/core/contracts/router"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
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

func TestPluginRegistry_IncludesBuiltinSecretManagers(t *testing.T) {
	b, err := New(Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}
	secretManagers := b.PluginRegistry().List(manifests.KindSecretManager)
	if len(secretManagers) == 0 {
		t.Fatal("expected built-in secret-manager manifests to be registered")
	}
}

func TestPluginRegistry_IncludesBuiltinStates(t *testing.T) {
	b, err := New(Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}
	states := b.PluginRegistry().List(manifests.KindState)
	if len(states) == 0 {
		t.Fatal("expected built-in state manifests to be registered")
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

func TestResolveRuntime_ExternalWhenPreferExternal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNFABRIC_HOME", home)
	writeExternalRuntime(t, home, "nodejs", "0.1.0")

	b, err := New(Options{IncludeExternal: true, PreferExternal: true})
	if err != nil {
		t.Fatalf("new boundary with external runtime: %v", err)
	}
	rt, err := b.ResolveRuntimePlugin("nodejs20.x")
	if err != nil {
		t.Fatalf("resolve runtime plugin nodejs20.x: %v", err)
	}
	if rt.Meta().ID != "nodejs" {
		t.Fatalf("runtime plugin id = %q, want nodejs", rt.Meta().ID)
	}
}

func TestResolveRuntime_ExternalOnlyUsesExternal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNFABRIC_HOME", home)
	writeExternalRuntime(t, home, "nodejs", "0.1.0")

	providerpolicy.ExternalOnlyRuntimes["nodejs"] = true
	t.Cleanup(func() { delete(providerpolicy.ExternalOnlyRuntimes, "nodejs") })

	b, err := New(Options{IncludeExternal: true})
	if err != nil {
		t.Fatalf("new boundary with external runtime: %v", err)
	}
	rt, err := b.ResolveRuntimePlugin("nodejs")
	if err != nil {
		t.Fatalf("resolve runtime plugin nodejs: %v", err)
	}
	if rt.Meta().ID != "nodejs" {
		t.Fatalf("runtime plugin id = %q, want nodejs", rt.Meta().ID)
	}
}

func TestResolveSimulator_ExternalWhenPreferExternal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNFABRIC_HOME", home)
	writeExternalSimulator(t, home, "local", "0.1.0")

	b, err := New(Options{IncludeExternal: true, PreferExternal: true})
	if err != nil {
		t.Fatalf("new boundary with external simulator: %v", err)
	}
	sim, err := b.ResolveSimulator("local")
	if err != nil {
		t.Fatalf("resolve simulator local: %v", err)
	}
	if sim.Meta().ID != "local" {
		t.Fatalf("simulator id = %q, want local", sim.Meta().ID)
	}
}

func TestResolveSecretManager_ExternalRegistration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNFABRIC_HOME", home)
	writeExternalSecretManager(t, home, "stub-sm", "0.1.0")

	b, err := New(Options{IncludeExternal: true})
	if err != nil {
		t.Fatalf("new boundary with external secret-manager: %v", err)
	}
	if _, err := b.ResolveSecretManager("stub-sm"); err != nil {
		t.Fatalf("resolve secret manager stub-sm: %v", err)
	}
}

func TestResolveStateBundleFactory_ExternalRegistration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNFABRIC_HOME", home)
	writeExternalState(t, home, "custom-state", "0.1.0", "custom")

	b, err := New(Options{IncludeExternal: true})
	if err != nil {
		t.Fatalf("new boundary with external state plugin: %v", err)
	}
	if _, err := b.ResolveStateBundleFactory("custom"); err != nil {
		t.Fatalf("resolve state bundle factory custom: %v", err)
	}
}
