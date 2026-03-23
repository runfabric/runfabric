package providerpolicy

import (
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
)

// BuiltinProviderFactory returns the constructor for a provider ID when the provider
// has an in-repo builtin implementation.
func BuiltinProviderFactory(id string) (func() providers.ProviderPlugin, bool) {
	lookupID := strings.TrimSpace(id)
	for _, entry := range providerEntries {
		if entry.Descriptor.ID != lookupID {
			continue
		}
		if !entry.Descriptor.BuiltinImplementation || entry.Factory == nil {
			return nil, false
		}
		return entry.Factory, true
	}
	return nil, false
}
