package resolution

import (
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func loadBuiltinStates(reg *manifests.PluginRegistry) {
	registerPluginMetaManifests(reg, manifests.KindState, providerpolicy.BuiltinStateManifests())
}
