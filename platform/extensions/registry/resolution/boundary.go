package resolution

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

// Boundary is the engine extension resolution boundary for provider/runtime resolution.
// App/bootstrap and other engine entry points should resolve extension capabilities through
// this type instead of constructing provider/runtime registries ad hoc.
type Boundary struct {
	providers          *providers.Registry
	runtimes           *runtimes.Registry
	simulators         *simulators.Registry
	plugins            *manifests.PluginRegistry
	internalProviderID map[string]struct{}
	apiProviderID      map[string]struct{}
	discoverOptions    external.DiscoverOptions
}

type Options struct {
	// IncludeExternal discovers and merges external plugins from RUNFABRIC_HOME/plugins.
	IncludeExternal bool
	// PreferExternal allows external plugins to override non-internal built-ins.
	PreferExternal bool
	// PinnedVersions optionally pins external plugin versions by plugin ID.
	PinnedVersions map[string]string
}

func New(opts Options) (*Boundary, error) {
	b := &Boundary{
		providers:  extproviders.NewBuiltinProvidersRegistry(),
		runtimes:   runtimes.NewBuiltinRegistry(),
		simulators: simulators.NewBuiltinRegistry(),
		plugins:    manifests.NewPluginRegistry(),
		discoverOptions: external.DiscoverOptions{
			PreferExternal: opts.PreferExternal,
			PinnedVersions: opts.PinnedVersions,
		},
		internalProviderID: map[string]struct{}{},
		apiProviderID:      map[string]struct{}{},
	}
	for _, name := range deployapi.APIProviderNames() {
		b.apiProviderID[name] = struct{}{}
	}

	// API-backed providers are part of provider resolution, even when lifecycle operations
	// dispatch through deployapi today.
	RegisterAPIProviders(b.providers)

	if opts.IncludeExternal {
		if err := b.RefreshExternal(); err != nil {
			return nil, err
		}
	}
	return b, nil
}

func (b *Boundary) ProviderRegistry() *providers.Registry {
	return b.providers
}

func (b *Boundary) PluginRegistry() *manifests.PluginRegistry {
	return b.plugins
}

func (b *Boundary) ResolveProvider(name string) (providers.ProviderPlugin, error) {
	id := strings.TrimSpace(name)
	p, ok := b.providers.Get(id)
	if !ok {
		return nil, providers.ErrProviderNotFound(id)
	}
	return p, nil
}

// ResolveRuntime resolves a runtime id/version string to a runtime plugin manifest.
// Runtime versions (e.g. nodejs20.x) are normalized to their runtime plugin IDs.
func (b *Boundary) ResolveRuntime(runtime string) (*manifests.PluginManifest, error) {
	raw := strings.TrimSpace(runtime)
	if raw == "" {
		return nil, fmt.Errorf("runtime is required")
	}
	id := runtimes.NormalizeRuntimeID(raw)
	m := b.plugins.Get(id)
	if m == nil || m.Kind != manifests.KindRuntime {
		return nil, fmt.Errorf("runtime plugin %q is not registered", raw)
	}
	return m, nil
}

func (b *Boundary) ResolveRuntimePlugin(runtime string) (runtimes.Runtime, error) {
	return b.runtimes.Get(runtime)
}

func (b *Boundary) ResolveSimulator(simulatorID string) (simulators.Simulator, error) {
	id := strings.TrimSpace(simulatorID)
	if id == "" {
		return nil, fmt.Errorf("simulator id is required")
	}
	return b.simulators.Get(id)
}

// IsInternalProvider returns true for providers that must remain engine-internal.
func (b *Boundary) IsInternalProvider(name string) bool {
	_, ok := b.internalProviderID[strings.TrimSpace(name)]
	return ok
}

func (b *Boundary) IsAPIDispatchProvider(name string) bool {
	_, ok := b.apiProviderID[strings.TrimSpace(name)]
	return ok
}

// RefreshExternal re-discovers external plugins and merges them into the boundary registries.
// Built-ins keep precedence on ID conflicts.
func (b *Boundary) RefreshExternal() error {
	res, err := external.Discover(b.discoverOptions)
	if err != nil {
		return err
	}
	for _, m := range res.Plugins {
		if m == nil {
			continue
		}
		// Built-in manifests keep precedence unless PreferExternal is enabled.
		if b.plugins.Get(m.ID) == nil || (b.discoverOptions.PreferExternal && !(m.Kind == manifests.KindProvider && b.IsInternalProvider(m.ID))) {
			b.plugins.Register(m)
		}
		if m.Kind != manifests.KindProvider || strings.TrimSpace(m.Executable) == "" {
			continue
		}
		// Keep internal providers authoritative while contract stabilizes.
		if b.IsInternalProvider(m.ID) {
			continue
		}
		if _, ok := b.providers.Get(m.ID); ok && !b.discoverOptions.PreferExternal {
			continue
		}
		_ = b.providers.Register(external.NewExternalProviderAdapter(m.ID, m.Executable, providers.ProviderMeta{
			Name:              m.ID,
			Capabilities:      append([]string(nil), m.Capabilities...),
			SupportsRuntime:   append([]string(nil), m.SupportsRuntime...),
			SupportsTriggers:  append([]string(nil), m.SupportsTriggers...),
			SupportsResources: append([]string(nil), m.SupportsResources...),
		}))
	}
	return nil
}
