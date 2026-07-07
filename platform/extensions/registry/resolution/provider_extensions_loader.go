package resolution

import (
	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func loadBuiltinProviders(reg *manifests.PluginRegistry) (*providers.Registry, map[string]struct{}) {
	providerSet := providerpolicy.NewBuiltinProviderSet()
	for _, provider := range providerSet.ManifestProviders {
		// Register the full built-in manifest (not just id/name/description) so the
		// catalog carries the provider's declared credential surface and init
		// scaffold — the PaaS reads these from GET /extensions.
		reg.Register(&manifests.PluginManifest{
			ID:          provider.ID,
			Kind:        manifests.KindProvider,
			Name:        provider.Name,
			Description: provider.Description,
			Credentials: manifests.CredentialSpecs(provider.Credentials),
			Scaffold:    manifests.ScaffoldSpecFrom(provider.Scaffold),
		})
	}
	providersRegistry := providerSet.Registry
	RegisterAPIProviders(providersRegistry)
	return providersRegistry, providerSet.APIProviderIDs
}
