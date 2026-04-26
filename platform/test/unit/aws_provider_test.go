package unit

import (
	"testing"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
	resolution "github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

// resolveBuiltinProvider resolves the first provider listed as a builtin implementation
// in the provider policy. Tests that need any concrete provider (not AWS-specific)
// should use this so the test stays valid regardless of which providers are built-in.
func resolveBuiltinProvider(t *testing.T) providers.ProviderPlugin {
	t.Helper()

	ids := providerpolicy.BuiltinImplementationIDs()
	if len(ids) == 0 {
		t.Skip("no builtin providers defined in policy")
	}

	boundary, err := resolution.New(resolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("create extension boundary: %v", err)
	}

	p, err := boundary.ResolveProvider(ids[0])
	if err != nil {
		t.Fatalf("resolve builtin provider %q: %v", ids[0], err)
	}
	return p
}
