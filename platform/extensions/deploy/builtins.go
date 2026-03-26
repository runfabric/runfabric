package deploy

import (
	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	extproviders "github.com/runfabric/runfabric/platform/extensions"
)

// NewBuiltinProvidersRegistry returns a provider registry populated with all built-in providers.
// Exposed here so packages outside platform/extensions/ can call it without violating internal rules.
func NewBuiltinProvidersRegistry() *providers.Registry {
	return extproviders.NewBuiltinProvidersRegistry()
}
