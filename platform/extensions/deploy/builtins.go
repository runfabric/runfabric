package deploy

import (
	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	extproviders "github.com/runfabric/runfabric/platform/extensions/internal/providers"
)

// NewBuiltinProvidersRegistry returns a provider registry populated with all built-in providers.
// Exposed here so packages outside platform/extensions/ can call it without violating internal rules.
func NewBuiltinProvidersRegistry() *providers.Registry {
	return extproviders.NewBuiltinProvidersRegistry()
}
