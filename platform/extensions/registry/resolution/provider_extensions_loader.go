package resolution

import (
	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func loadBuiltinProviders(reg *manifests.PluginRegistry) (*providers.Registry, map[string]struct{}) {
	providerSet := providerpolicy.NewBuiltinProviderSet()
	for _, provider := range providerSet.ManifestProviders {
		registerManifest(reg, manifests.KindProvider, provider.ID, provider.Name, provider.Description)
	}
	providersRegistry := providerSet.Registry
	RegisterAPIProviders(providersRegistry)
	return providersRegistry, providerSet.APIProviderIDs
}
