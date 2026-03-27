package resolution

import (
	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

// BuiltinSet contains the built-in extension registries and metadata.
type BuiltinSet struct {
	Providers      *providers.Registry
	Runtimes       *providerpolicy.RuntimeRegistry
	Simulators     *providerpolicy.SimulatorRegistry
	Routers        *providerpolicy.RouterRegistry
	Plugins        *manifests.PluginRegistry
	APIProviderIDs map[string]struct{}
}

// NewBuiltinSet centralizes built-in extension loading for providers, runtimes, and simulators.
func NewBuiltinSet() *BuiltinSet {
	providerSet := providerpolicy.NewBuiltinProviderSet()
	reg := manifests.NewEmptyPluginRegistry()
	for _, provider := range providerSet.ManifestProviders {
		reg.Register(&manifests.PluginManifest{ID: provider.ID, Kind: manifests.KindProvider, Name: provider.Name, Description: provider.Description})
	}
	for _, rt := range providerpolicy.BuiltinRuntimeManifests() {
		reg.Register(&manifests.PluginManifest{ID: rt.ID, Kind: manifests.KindRuntime, Name: rt.Name, Description: rt.Description})
	}
	for _, sim := range providerpolicy.BuiltinSimulatorManifests() {
		reg.Register(&manifests.PluginManifest{ID: sim.ID, Kind: manifests.KindSimulator, Name: sim.Name, Description: sim.Description})
	}
	for _, router := range providerpolicy.BuiltinRouterManifests() {
		reg.Register(&manifests.PluginManifest{ID: router.ID, Kind: manifests.KindRouter, Name: router.Name, Description: router.Description})
	}

	providersRegistry := providerSet.Registry
	RegisterAPIProviders(providersRegistry)

	return &BuiltinSet{
		Providers:      providersRegistry,
		Runtimes:       providerpolicy.NewBuiltinRuntimeRegistry(),
		Simulators:     providerpolicy.NewBuiltinSimulatorRegistry(),
		Routers:        providerpolicy.NewBuiltinRouterRegistry(),
		Plugins:        reg,
		APIProviderIDs: providerSet.APIProviderIDs,
	}
}

func NormalizeRuntimeID(runtime string) string {
	return providerpolicy.NormalizeRuntimeID(runtime)
}
