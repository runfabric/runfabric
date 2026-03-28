package resolution

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	routercontracts "github.com/runfabric/runfabric/platform/core/contracts/router"
	runtimecontracts "github.com/runfabric/runfabric/platform/core/contracts/runtime"
	simulatorcontracts "github.com/runfabric/runfabric/platform/core/contracts/simulators"
	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/extensions/application/external"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

// Boundary is the engine extension resolution boundary for provider/runtime resolution.
// App/bootstrap and other engine entry points should resolve extension capabilities through
// this type instead of constructing provider/runtime registries ad hoc.
type Boundary struct {
	providers          *providers.Registry
	runtimes           providerpolicy.RuntimeRegistry
	simulators         providerpolicy.SimulatorRegistry
	routers            providerpolicy.RouterRegistry
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

// RuntimeBuildRequest contains all inputs needed to build a single function via the resolved runtime plugin.
type RuntimeBuildRequest struct {
	Runtime         string
	Root            string
	FunctionName    string
	FunctionConfig  config.FunctionConfig
	ConfigSignature string
}

// SimulatorInvokeRequest captures local invoke inputs for simulator execution.
type SimulatorInvokeRequest struct {
	Service    string
	Stage      string
	Function   string
	Method     string
	Path       string
	Query      map[string]string
	Headers    map[string]string
	Body       []byte
	WorkDir    string
	HandlerRef string
	Runtime    string
}

// SimulatorInvokeResult is the normalized simulator response returned by the boundary.
type SimulatorInvokeResult struct {
	StatusCode int
	Headers    map[string]string
	Body       json.RawMessage
}

func New(opts Options) (*Boundary, error) {
	return newBoundary(opts)
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
	id := NormalizeRuntimeID(raw)
	m := b.plugins.Get(id)
	if m == nil || m.Kind != manifests.KindRuntime {
		return nil, fmt.Errorf("runtime plugin %q is not registered", raw)
	}
	return m, nil
}

func (b *Boundary) ResolveRuntimePlugin(runtime string) (runtimecontracts.Runtime, error) {
	return b.runtimes.Get(runtime)
}

func (b *Boundary) ResolveSimulator(simulatorID string) (simulatorcontracts.Simulator, error) {
	id := strings.TrimSpace(simulatorID)
	if id == "" {
		return nil, fmt.Errorf("simulator id is required")
	}
	return b.simulators.Get(id)
}

func (b *Boundary) ResolveRouter(routerID string) (routercontracts.Router, error) {
	id := strings.TrimSpace(routerID)
	if id == "" {
		return nil, fmt.Errorf("router id is required")
	}
	return b.routers.Get(id)
}

func (b *Boundary) SyncRouter(ctx context.Context, routerID string, req routercontracts.SyncRequest) (*routercontracts.SyncResult, error) {
	router, err := b.ResolveRouter(routerID)
	if err != nil {
		return nil, err
	}
	return router.Sync(ctx, req)
}

// BuildFunction resolves the runtime plugin and builds a single function artifact.
func (b *Boundary) BuildFunction(ctx context.Context, req RuntimeBuildRequest) (*providers.Artifact, error) {
	runtimePlugin, err := b.ResolveRuntimePlugin(req.Runtime)
	if err != nil {
		return nil, err
	}
	artifact, err := runtimePlugin.Build(ctx, runtimecontracts.BuildRequest{
		Root:            req.Root,
		FunctionName:    req.FunctionName,
		FunctionConfig:  req.FunctionConfig,
		ConfigSignature: req.ConfigSignature,
	})
	if err != nil {
		return nil, err
	}
	return artifact, nil
}

// Simulate resolves the simulator plugin and runs one local invoke request.
func (b *Boundary) Simulate(ctx context.Context, simulatorID string, req SimulatorInvokeRequest) (*SimulatorInvokeResult, error) {
	simulator, err := b.ResolveSimulator(simulatorID)
	if err != nil {
		return nil, err
	}
	res, err := simulator.Simulate(ctx, simulatorcontracts.Request{
		Service:    req.Service,
		Stage:      req.Stage,
		Function:   req.Function,
		Method:     req.Method,
		Path:       req.Path,
		Query:      req.Query,
		Headers:    req.Headers,
		Body:       req.Body,
		WorkDir:    req.WorkDir,
		HandlerRef: req.HandlerRef,
		Runtime:    req.Runtime,
	})
	if err != nil {
		return nil, err
	}
	return &SimulatorInvokeResult{
		StatusCode: res.StatusCode,
		Headers:    res.Headers,
		Body:       res.Body,
	}, nil
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
		if strings.TrimSpace(m.Executable) == "" {
			continue
		}
		switch m.Kind {
		case manifests.KindProvider:
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
		case manifests.KindRouter:
			if _, err := b.routers.Get(m.ID); err == nil && !b.discoverOptions.PreferExternal {
				continue
			}
			_ = b.routers.Register(external.NewExternalRouterAdapter(m.ID, m.Executable, routercontracts.PluginMeta{
				ID: m.ID, Name: m.Name, Version: m.Version, Description: m.Description,
			}))
		}
	}
	return nil
}
