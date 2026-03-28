package resolution

import (
	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func loadBuiltinProviders(reg *manifests.PluginRegistry) (*providers.Registry, map[string]struct{}) {
	providerSet := providerpolicy.NewBuiltinProviderSet()
	for _, provider := range providerSet.ManifestProviders {
		reg.Register(&manifests.PluginManifest{
			ID:          provider.ID,
			Kind:        manifests.KindProvider,
			Name:        provider.Name,
			Description: provider.Description,
		})
	}
	providersRegistry := providerSet.Registry
	RegisterAPIProviders(providersRegistry)
	return providersRegistry, providerSet.APIProviderIDs
}
