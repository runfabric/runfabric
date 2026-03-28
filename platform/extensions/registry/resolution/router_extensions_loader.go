package resolution

import (
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func loadBuiltinRouters(reg *manifests.PluginRegistry) providerpolicy.RouterRegistry {
	registerPluginMetaManifests(reg, manifests.KindRouter, providerpolicy.BuiltinRouterManifests())
	return providerpolicy.NewBuiltinRouterRegistry()
}
