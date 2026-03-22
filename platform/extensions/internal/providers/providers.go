package providers

import (
	extproviders "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	awsprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/aws"
	gcpprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/gcp"
)

// NewBuiltinProvidersRegistry builds a provider registry populated with all built-in
// provider implementations while keeping the "loading orchestration" in platform/extensions.
func NewBuiltinProvidersRegistry() *extproviders.Registry {
	reg := extproviders.NewRegistry()
	_ = reg.Register(awsprovider.New())
	_ = reg.Register(gcpprovider.New())
	return reg
}
