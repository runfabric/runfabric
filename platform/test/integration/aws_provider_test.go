package integration

import (
	"testing"

	provider "github.com/runfabric/runfabric/internal/provider/contracts"
	resolution "github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

func resolveAWSProvider(t *testing.T) provider.ProviderPlugin {
	t.Helper()

	boundary, err := resolution.New(resolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("create extension boundary: %v", err)
	}

	p, err := boundary.ResolveProvider("aws-lambda")
	if err != nil {
		t.Fatalf("resolve aws-lambda provider: %v", err)
	}
	return p
}
