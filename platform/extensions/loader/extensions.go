// Package loader provides a single consolidated entry point for loading all
// runfabric extensions (providers, runtimes, simulators) including both built-in
// and external plugins from RUNFABRIC_HOME.
package loader

import (
	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	runtimecontracts "github.com/runfabric/runfabric/platform/core/contracts/runtime"
	simulatorcontracts "github.com/runfabric/runfabric/platform/core/contracts/simulators"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
	providerloader "github.com/runfabric/runfabric/platform/extensions/registry/loader/providers"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

// ExtensionsLoader is the single consolidated entry point for loading all extensions.
// Implementation is delegated to the shared resolution boundary so extension loading
// logic lives in one common place.
type ExtensionsLoader struct {
	boundary *resolution.Boundary
}

// LoaderOptions configures extension discovery behavior.
type LoaderOptions struct {
	// IncludeExternal discovers external plugins from RUNFABRIC_HOME/plugins.
	IncludeExternal bool
	// PreferExternal allows external plugins to override built-in providers with same ID.
	PreferExternal bool
	// PinnedVersions pins specific versions for external plugins by ID.
	// Example: {"aws-lambda": "1.2.0", "vercel": "2.0.0"}
	PinnedVersions map[string]string
}

// NewLoader creates a new extensions loader with all built-in providers, runtimes,
// and simulators. If IncludeExternal is true, it also discovers plugins from
// RUNFABRIC_HOME/plugins directories.
func NewLoader(opts LoaderOptions) (*ExtensionsLoader, error) {
	boundary, err := providerloader.LoadBoundary(providerloader.LoadOptions{
		IncludeExternal: opts.IncludeExternal,
		PreferExternal:  opts.PreferExternal,
		PinnedVersions:  opts.PinnedVersions,
	})
	if err != nil {
		return nil, err
	}
	return &ExtensionsLoader{boundary: boundary}, nil
}

// Providers returns the providers registry containing all built-in and (optionally) external providers.
func (l *ExtensionsLoader) Providers() *providers.Registry {
	return l.boundary.ProviderRegistry()
}

// Runtimes returns the runtimes registry containing all built-in runtime plugins.
func (l *ExtensionsLoader) Runtimes() providerpolicy.RuntimeRegistry {
	return boundaryRuntimeRegistry{boundary: l.boundary}
}

// Simulators returns the simulators registry containing all built-in simulator plugins.
func (l *ExtensionsLoader) Simulators() providerpolicy.SimulatorRegistry {
	return boundarySimulatorRegistry{boundary: l.boundary}
}

// Plugins returns the plugin manifest registry containing metadata for all discovered plugins.
func (l *ExtensionsLoader) Plugins() *manifests.PluginRegistry {
	return l.boundary.PluginRegistry()
}

// ResolveProvider returns a provider by name, or error if not found.
func (l *ExtensionsLoader) ResolveProvider(name string) (providers.ProviderPlugin, error) {
	return l.boundary.ResolveProvider(name)
}

// ResolveRuntime normalizes and resolves a runtime ID/version to a plugin manifest.
// Examples: "nodejs20.x" → normalized to "nodejs"
func (l *ExtensionsLoader) ResolveRuntime(runtime string) (*manifests.PluginManifest, error) {
	return l.boundary.ResolveRuntime(runtime)
}

// ResolveRuntimePlugin returns a runtime by ID/version string.
func (l *ExtensionsLoader) ResolveRuntimePlugin(runtime string) (runtimecontracts.Runtime, error) {
	return l.boundary.ResolveRuntimePlugin(runtime)
}

// ResolveSimulator returns a simulator by ID.
func (l *ExtensionsLoader) ResolveSimulator(simulatorID string) (simulatorcontracts.Simulator, error) {
	return l.boundary.ResolveSimulator(simulatorID)
}

// IsInternalProvider returns true if the provider is marked as internal (non-external).
func (l *ExtensionsLoader) IsInternalProvider(name string) bool {
	return l.boundary.IsInternalProvider(name)
}

// IsAPIProvider returns true if the provider is API-backed (dispatches through deploy/api).
func (l *ExtensionsLoader) IsAPIProvider(name string) bool {
	return l.boundary.IsAPIDispatchProvider(name)
}

// Refresh re-discovers external plugins and merges them into the loader registries.
// Call this to hot-reload external plugins after installing/updating them.
func (l *ExtensionsLoader) Refresh() error {
	return l.boundary.RefreshExternal()
}

type boundaryRuntimeRegistry struct {
	boundary *resolution.Boundary
}

func (r boundaryRuntimeRegistry) Get(runtime string) (runtimecontracts.Runtime, error) {
	return r.boundary.ResolveRuntimePlugin(runtime)
}

func (r boundaryRuntimeRegistry) Register(runtime runtimecontracts.Runtime) error {
	return r.boundary.RegisterRuntime(runtime)
}

type boundarySimulatorRegistry struct {
	boundary *resolution.Boundary
}

func (r boundarySimulatorRegistry) Get(simulatorID string) (simulatorcontracts.Simulator, error) {
	return r.boundary.ResolveSimulator(simulatorID)
}

func (r boundarySimulatorRegistry) Register(simulator simulatorcontracts.Simulator) error {
	return r.boundary.RegisterSimulator(simulator)
}

// DefaultLoader creates a loader with external plugins enabled and no version pinning.
func DefaultLoader() (*ExtensionsLoader, error) {
	return NewLoader(LoaderOptions{
		IncludeExternal: true,
		PreferExternal:  false,
		PinnedVersions:  nil,
	})
}

// BuiltinOnlyLoader creates a loader with only built-in extensions (no external plugins).
func BuiltinOnlyLoader() (*ExtensionsLoader, error) {
	return NewLoader(LoaderOptions{
		IncludeExternal: false,
	})
}
