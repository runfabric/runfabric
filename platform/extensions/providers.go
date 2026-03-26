package extensions

import (
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	extproviders "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

// NewBuiltinProvidersRegistry builds a provider registry populated with all built-in
// provider implementations while keeping the "loading orchestration" in platform/extensions.
func NewBuiltinProvidersRegistry() *extproviders.Registry {
	reg := extproviders.NewRegistry()
	for _, id := range providerpolicy.BuiltinImplementationIDs() {
		if create, ok := providerpolicy.BuiltinProviderFactory(id); ok {
			_ = reg.Register(inprocess.New(create()))
		}
	}
	return reg
}
