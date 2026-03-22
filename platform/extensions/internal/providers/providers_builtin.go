package providers

import (
	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	awsprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/aws"
	gcpprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/gcp"
)

// NewBuiltinProviderRegistry returns a providers.Registry populated with all built-in
// provider plugins (AWS, GCP). Built-in providers are part of the extensions model;
// bootstrap and lifecycle use this registry to resolve provider by config name.
func NewBuiltinProviderRegistry() *providers.Registry {
	reg := providers.NewRegistry()
	_ = reg.Register(awsprovider.New())
	_ = reg.Register(gcpprovider.New())
	return reg
}
