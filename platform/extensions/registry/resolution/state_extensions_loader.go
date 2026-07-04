package resolution

import (
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func loadBuiltinStates(reg *manifests.PluginRegistry) {
	// Like providers, state backends declare their credential env vars once
	// (extensions/states BuiltinStateCredentials) and the manifest surfaces
	// them for plugins list/info.
	for _, item := range providerpolicy.BuiltinStateManifests() {
		reg.Register(&manifests.PluginManifest{
			ID:          item.ID,
			Kind:        manifests.KindState,
			Name:        item.Name,
			Description: item.Description,
			Credentials: manifests.CredentialSpecs(providerpolicy.StateBackendCredentials(item.ID)),
		})
	}
}
