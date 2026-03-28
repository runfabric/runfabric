package resolution

import (
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func loadBuiltinRouters(reg *manifests.PluginRegistry) providerpolicy.RouterRegistry {
	for _, router := range providerpolicy.BuiltinRouterManifests() {
		reg.Register(&manifests.PluginManifest{
			ID:          router.ID,
			Kind:        manifests.KindRouter,
			Name:        router.Name,
			Description: router.Description,
		})
	}
	return providerpolicy.NewBuiltinRouterRegistry()
}
