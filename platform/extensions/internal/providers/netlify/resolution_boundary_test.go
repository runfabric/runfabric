package netlify_test

import (
	"testing"

	legacyresolution "github.com/runfabric/runfabric/platform/extension/resolution"
	registryresolution "github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

func TestNetlifyResolution_IsResolvedAndAPIDispatch(t *testing.T) {
	legacy, err := legacyresolution.New(legacyresolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("legacy boundary: %v", err)
	}
	if _, err := legacy.ResolveProvider("netlify"); err != nil {
		t.Fatalf("legacy resolve netlify: %v", err)
	}
	if !legacy.IsAPIDispatchProvider("netlify") {
		t.Fatal("legacy expected netlify to be API-dispatched")
	}

	registry, err := registryresolution.New(registryresolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("registry boundary: %v", err)
	}
	if _, err := registry.ResolveProvider("netlify"); err != nil {
		t.Fatalf("registry resolve netlify: %v", err)
	}
	if !registry.IsAPIDispatchProvider("netlify") {
		t.Fatal("registry expected netlify to be API-dispatched")
	}
}
