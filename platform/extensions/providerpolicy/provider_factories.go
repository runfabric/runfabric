package providerpolicy

import (
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// BuiltinProviderFactory returns the constructor for a provider ID when the provider
// has an in-repo builtin implementation.
func BuiltinProviderFactory(id string) (func() sdkprovider.Plugin, bool) {
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
