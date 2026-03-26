package resolution

import (
	builtinruntimes "github.com/runfabric/runfabric/extensions/runtimes"
	builtinsimulators "github.com/runfabric/runfabric/extensions/simulators"
	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

type RuntimeRegistry = builtinruntimes.Registry
type RuntimePlugin = builtinruntimes.Runtime
type RuntimeBuildInput = builtinruntimes.BuildRequest
type RuntimeFunctionSpec = builtinruntimes.FunctionSpec
type SimulatorRegistry = builtinsimulators.Registry
type SimulatorPlugin = builtinsimulators.Simulator
type SimulatorInput = builtinsimulators.Request

// BuiltinSet contains the built-in extension registries and metadata.
type BuiltinSet struct {
	Providers      *providers.Registry
	Runtimes       *RuntimeRegistry
	Simulators     *SimulatorRegistry
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
	for _, rt := range builtinruntimes.BuiltinRuntimeManifests() {
		reg.Register(&manifests.PluginManifest{ID: rt.ID, Kind: manifests.KindRuntime, Name: rt.Name, Description: rt.Description})
	}
	for _, sim := range builtinsimulators.BuiltinSimulatorManifests() {
		reg.Register(&manifests.PluginManifest{ID: sim.ID, Kind: manifests.KindSimulator, Name: sim.Name, Description: sim.Description})
	}

	providersRegistry := providerSet.Registry
	RegisterAPIProviders(providersRegistry)

	return &BuiltinSet{
		Providers:      providersRegistry,
		Runtimes:       builtinruntimes.NewBuiltinRegistry(),
		Simulators:     builtinsimulators.NewBuiltinRegistry(),
		Plugins:        reg,
		APIProviderIDs: providerSet.APIProviderIDs,
	}
}

func NormalizeRuntimeID(runtime string) string {
	return builtinruntimes.NormalizeRuntimeID(runtime)
}
