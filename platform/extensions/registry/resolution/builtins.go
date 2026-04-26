package resolution

import (
	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

// BuiltinSet contains the built-in extension registries and metadata.
type BuiltinSet struct {
	Providers      *providers.Registry
	Runtimes       providerpolicy.RuntimeRegistry
	Simulators     providerpolicy.SimulatorRegistry
	Routers        providerpolicy.RouterRegistry
	Plugins        *manifests.PluginRegistry
	APIProviderIDs map[string]struct{}
}

// NewBuiltinSet centralizes built-in extension loading for providers, runtimes, and simulators.
func NewBuiltinSet() *BuiltinSet {
	reg := manifests.NewEmptyPluginRegistry()
	providersRegistry, apiProviderIDs := loadBuiltinProviders(reg)
	runtimesRegistry := loadBuiltinRuntimes(reg)
	simulatorsRegistry := loadBuiltinSimulators(reg)
	routersRegistry := loadBuiltinRouters(reg)
	loadBuiltinSecretManagers(reg)
	loadBuiltinStates(reg)

	return &BuiltinSet{
		Providers:      providersRegistry,
		Runtimes:       runtimesRegistry,
		Simulators:     simulatorsRegistry,
		Routers:        routersRegistry,
		Plugins:        reg,
		APIProviderIDs: apiProviderIDs,
	}
}
