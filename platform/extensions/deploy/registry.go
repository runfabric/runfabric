package deploy

import (
	"sort"

	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

var apiProviders = func() map[string]Provider {
	providerSet := providerpolicy.NewBuiltinProviderSet()
	m := make(map[string]Provider, len(providerSet.APIDispatch))
	for id, provider := range providerSet.APIDispatch {
		m[id] = newAPIProviderFromOps(id, provider.Ops, provider.Hooks)
	}
	return m
}()

// GetProvider returns the API-dispatch provider for name, or nil, false if not found.
func GetProvider(name string) (Provider, bool) {
	p, ok := apiProviders[name]
	return p, ok
}

// HasProvider returns whether name has an API-dispatch provider.
func HasProvider(name string) bool {
	_, ok := apiProviders[name]
	return ok
}

// APIProviderNames returns the sorted list of API-dispatch provider names.
func APIProviderNames() []string {
	names := make([]string, 0, len(apiProviders))
	for k := range apiProviders {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
