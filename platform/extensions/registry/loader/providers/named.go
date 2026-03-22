package providers

import (
	ext "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
)

// NewNamedPlugin returns a ProviderPlugin that delegates to p but reports Meta().Name as name.
func NewNamedPlugin(name string, p ProviderPlugin) ProviderPlugin {
	return ext.NewNamedPlugin(name, p)
}
