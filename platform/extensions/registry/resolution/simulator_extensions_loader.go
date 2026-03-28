package resolution

import (
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func loadBuiltinSimulators(reg *manifests.PluginRegistry) providerpolicy.SimulatorRegistry {
	for _, sim := range providerpolicy.BuiltinSimulatorManifests() {
		reg.Register(&manifests.PluginManifest{
			ID:          sim.ID,
			Kind:        manifests.KindSimulator,
			Name:        sim.Name,
			Description: sim.Description,
		})
	}
	return providerpolicy.NewBuiltinSimulatorRegistry()
}
