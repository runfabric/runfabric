package providers

import (
	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
)

// NewBuiltinProviderRegistry returns a providers.Registry populated with all built-in
// provider plugins. Built-in providers are part of the extensions model;
// bootstrap and lifecycle use this registry to resolve provider by config name.
func NewBuiltinProviderRegistry() *providers.Registry {
	// Delegate to the canonical built-in registry so provider add/remove happens in one place.
	return NewBuiltinProvidersRegistry()
}
