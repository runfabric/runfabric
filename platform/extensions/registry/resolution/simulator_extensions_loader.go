package resolution

import (
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func loadBuiltinSimulators(reg *manifests.PluginRegistry) providerpolicy.SimulatorRegistry {
	registerPluginMetaManifests(reg, manifests.KindSimulator, providerpolicy.BuiltinSimulatorManifests())
	return providerpolicy.NewBuiltinSimulatorRegistry()
}
