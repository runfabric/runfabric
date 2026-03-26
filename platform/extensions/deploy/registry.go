package deploy

import (
	"sort"

	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

var apiProviders = func() map[string]Provider {
	m := make(map[string]Provider)
	for _, id := range providerpolicy.APIDispatchProviderIDs() {
		m[id] = buildProvider(id)
	}
	return m
}()

func buildProvider(id string) Provider {
	ops, _ := providerpolicy.GetProviderAPIOps(id)
	return newAPIProviderFromOps(id, ops, providerpolicy.GetAPIHooks(id))
}

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
