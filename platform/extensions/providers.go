package extensions

import (
	extproviders "github.com/runfabric/runfabric/platform/core/contracts/provider"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

// NewBuiltinProvidersRegistry builds a provider registry populated with all built-in
// provider implementations while keeping the "loading orchestration" in platform/extensions.
func NewBuiltinProvidersRegistry() *extproviders.Registry {
	return providerpolicy.NewBuiltinProviderSet().Registry
}
