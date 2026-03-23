// Package loader provides a single consolidated entry point for loading all
// runfabric extensions (providers, runtimes, simulators) including both built-in
// and external plugins from RUNFABRIC_HOME.
package loader

import (
	"fmt"
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	runtimes "github.com/runfabric/runfabric/platform/core/contracts/runtime"
	simulators "github.com/runfabric/runfabric/platform/core/contracts/simulators"
	deployapi "github.com/runfabric/runfabric/platform/deploy/core/api"
	"github.com/runfabric/runfabric/platform/extensions/application/external"
	extproviders "github.com/runfabric/runfabric/platform/extensions/internal/providers"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
)

// ExtensionsLoader is the single consolidated entry point for loading all extensions.
// It handles:
// - Built-in providers (gcp-functions, kubernetes, etc.)
// - API-backed providers (vercel, netlify, etc.)
// - Built-in runtimes (nodejs, python, go, etc.)
// - Built-in simulators (local, etc.)
// - External plugins from RUNFABRIC_HOME/plugins/{providers,runtimes,simulators}
type ExtensionsLoader struct {
	providers         *providers.Registry
	runtimes          *runtimes.Registry
	simulators        *simulators.Registry
	plugins           *manifests.PluginRegistry
	internalProviders map[string]struct{}
	apiProviders      map[string]struct{}
	discoverOptions   external.DiscoverOptions
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
	loader := &ExtensionsLoader{
		// Register all built-in registries
		providers:         extproviders.NewBuiltinProvidersRegistry(),
		runtimes:          runtimes.NewBuiltinRegistry(),
		simulators:        simulators.NewBuiltinRegistry(),
		plugins:           manifests.NewPluginRegistry(),
		internalProviders: map[string]struct{}{},
		apiProviders:      map[string]struct{}{},
		discoverOptions: external.DiscoverOptions{
			PreferExternal: opts.PreferExternal,
			PinnedVersions: opts.PinnedVersions,
		},
	}

	// Mark API providers (vercel, netlify, etc.) that dispatch through deploy/api
	for _, name := range deployapi.APIProviderNames() {
		loader.apiProviders[name] = struct{}{}
	}

	// Register API-backed providers to the provider registry
	registerBuiltinAPIProviders(loader.providers)

	// Load external plugins if requested
	if opts.IncludeExternal {
		if err := loader.refresh(); err != nil {
			return nil, err
		}
	}

	return loader, nil
}

// Providers returns the providers registry containing all built-in and (optionally) external providers.
func (l *ExtensionsLoader) Providers() *providers.Registry {
	return l.providers
}

// Runtimes returns the runtimes registry containing all built-in runtime plugins.
func (l *ExtensionsLoader) Runtimes() *runtimes.Registry {
	return l.runtimes
}

// Simulators returns the simulators registry containing all built-in simulators.
func (l *ExtensionsLoader) Simulators() *simulators.Registry {
	return l.simulators
}

// Plugins returns the plugin manifest registry containing metadata for all discovered plugins.
func (l *ExtensionsLoader) Plugins() *manifests.PluginRegistry {
	return l.plugins
}

// ResolveProvider returns a provider by name, or error if not found.
func (l *ExtensionsLoader) ResolveProvider(name string) (providers.ProviderPlugin, error) {
	id := strings.TrimSpace(name)
	p, ok := l.providers.Get(id)
	if !ok {
		return nil, providers.ErrProviderNotFound(id)
	}
	return p, nil
}

// ResolveRuntime normalizes and resolves a runtime ID/version to a plugin manifest.
// Examples: "nodejs20.x" → normalized to "nodejs"
func (l *ExtensionsLoader) ResolveRuntime(runtime string) (*manifests.PluginManifest, error) {
	raw := strings.TrimSpace(runtime)
	if raw == "" {
		return nil, fmt.Errorf("runtime is required")
	}
	id := runtimes.NormalizeRuntimeID(raw)
	m := l.plugins.Get(id)
	if m == nil || m.Kind != manifests.KindRuntime {
		return nil, fmt.Errorf("runtime plugin %q is not registered", raw)
	}
	return m, nil
}

// ResolveRuntimePlugin returns a runtime by ID/version string.
func (l *ExtensionsLoader) ResolveRuntimePlugin(runtime string) (runtimes.Runtime, error) {
	return l.runtimes.Get(runtime)
}

// ResolveSimulator returns a simulator by ID.
func (l *ExtensionsLoader) ResolveSimulator(simulatorID string) (simulators.Simulator, error) {
	id := strings.TrimSpace(simulatorID)
	if id == "" {
		return nil, fmt.Errorf("simulator id is required")
	}
	return l.simulators.Get(id)
}

// IsInternalProvider returns true if the provider is marked as internal (non-external).
func (l *ExtensionsLoader) IsInternalProvider(name string) bool {
	_, ok := l.internalProviders[strings.TrimSpace(name)]
	return ok
}

// IsAPIProvider returns true if the provider is API-backed (dispatches through deploy/api).
func (l *ExtensionsLoader) IsAPIProvider(name string) bool {
	_, ok := l.apiProviders[strings.TrimSpace(name)]
	return ok
}

// Refresh re-discovers external plugins and merges them into the loader registries.
// Call this to hot-reload external plugins after installing/updating them.
func (l *ExtensionsLoader) Refresh() error {
	return l.refresh()
}

// refresh does the actual external plugin discovery and registration.
func (l *ExtensionsLoader) refresh() error {
	res, err := external.Discover(l.discoverOptions)
	if err != nil {
		return err
	}

	// Register plugin manifests
	for _, m := range res.Plugins {
		if m == nil {
			continue
		}
		// Built-in manifests keep precedence unless PreferExternal is enabled
		if l.plugins.Get(m.ID) == nil || (l.discoverOptions.PreferExternal && !(m.Kind == manifests.KindProvider && l.IsInternalProvider(m.ID))) {
			l.plugins.Register(m)
		}
	}

	// Register provider adapters for external provider plugins
	for _, m := range res.Plugins {
		if m == nil || m.Kind != manifests.KindProvider || strings.TrimSpace(m.Executable) == "" {
			continue
		}

		// Skip if marked as internal provider (non-external)
		if l.IsInternalProvider(m.ID) {
			continue
		}

		// Skip if already registered and not preferring external
		if _, ok := l.providers.Get(m.ID); ok && !l.discoverOptions.PreferExternal {
			continue
		}

		// Register external provider as subprocess adapter
		_ = l.providers.Register(external.NewExternalProviderAdapter(
			m.ID,
			m.Executable,
			providers.ProviderMeta{
				Name:              m.ID,
				Capabilities:      append([]string(nil), m.Capabilities...),
				SupportsRuntime:   append([]string(nil), m.SupportsRuntime...),
				SupportsTriggers:  append([]string(nil), m.SupportsTriggers...),
				SupportsResources: append([]string(nil), m.SupportsResources...),
			},
		))
	}

	return nil
}

// registerBuiltinAPIProviders registers all API-backed providers (vercel, netlify, etc.)
// to the provider registry. These providers dispatch through the API instead of
// direct Go implementations.
func registerBuiltinAPIProviders(reg *providers.Registry) {
	// Placeholder: API providers are registered via deployapi contract.
	// Implementation details in platform/deploy/core/api/providers.go
	// Each API provider (vercel, netlify, etc.) is registered with its RPC endpoint.
	_ = reg // Use reg to avoid unused variable warning
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
