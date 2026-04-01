package integration

import (
	"strings"
	"testing"

	provider "github.com/runfabric/runfabric/internal/provider/contracts"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	resolution "github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

func testProviderNameAndRuntime(t *testing.T) (string, string) {
	t.Helper()
	boundary, err := resolution.New(resolution.Options{IncludeExternal: false})
	if err == nil {
		providers := boundary.PluginRegistry().List(manifests.KindProvider)
		for _, p := range providers {
			if p == nil {
				continue
			}
			if id := strings.TrimSpace(p.ID); id != "" {
				return id, "nodejs20.x"
			}
		}
	}
	return "aws-lambda", "nodejs20.x"
}

func resolveTestProvider(t *testing.T) provider.ProviderPlugin {
	t.Helper()
	providerID, _ := testProviderNameAndRuntime(t)

	boundary, err := resolution.New(resolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("create extension boundary: %v", err)
	}

	p, err := boundary.ResolveProvider(providerID)
	if err != nil {
		t.Fatalf("resolve provider %q: %v", providerID, err)
	}
	return p
}
