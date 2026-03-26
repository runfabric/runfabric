package contracts

import provider "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"

// Registry holds provider plugins by name. Alias of the canonical definition.
type Registry = provider.Registry

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return provider.NewRegistry()
}
