package resolution

import (
	routercontracts "github.com/runfabric/runfabric/platform/core/contracts/router"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
)

func registerManifest(reg *manifests.PluginRegistry, kind manifests.PluginKind, id, name, description string) {
	reg.Register(&manifests.PluginManifest{
		ID:          id,
		Kind:        kind,
		Name:        name,
		Description: description,
	})
}

func registerPluginMetaManifests(reg *manifests.PluginRegistry, kind manifests.PluginKind, items []routercontracts.PluginMeta) {
	for _, item := range items {
		registerManifest(reg, kind, item.ID, item.Name, item.Description)
	}
}
