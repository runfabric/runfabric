package gcp_test

import (
	"testing"

	legacyresolution "github.com/runfabric/runfabric/platform/extension/resolution"
	registryresolution "github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

func TestGCPResolution_IsResolvedAndAPIDispatch(t *testing.T) {
	legacy, err := legacyresolution.New(legacyresolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("legacy boundary: %v", err)
	}
	if _, err := legacy.ResolveProvider("gcp-functions"); err != nil {
		t.Fatalf("legacy resolve gcp-functions: %v", err)
	}
	if !legacy.IsAPIDispatchProvider("gcp-functions") {
		t.Fatal("legacy expected gcp-functions to be API-dispatched")
	}

	registry, err := registryresolution.New(registryresolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("registry boundary: %v", err)
	}
	if _, err := registry.ResolveProvider("gcp-functions"); err != nil {
		t.Fatalf("registry resolve gcp-functions: %v", err)
	}
	if !registry.IsAPIDispatchProvider("gcp-functions") {
		t.Fatal("registry expected gcp-functions to be API-dispatched")
	}
}
