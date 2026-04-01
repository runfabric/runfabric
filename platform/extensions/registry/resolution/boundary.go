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
	"github.com/runfabric/runfabric/platform/state/backends"
)

// Boundary is the engine extension resolution boundary for provider/runtime resolution.
// App/bootstrap and other engine entry points should resolve extension capabilities through
// this type instead of constructing provider/runtime registries ad hoc.
type Boundary struct {
	providers          *providers.Registry
	runtimes           providerpolicy.RuntimeRegistry
	simulators         providerpolicy.SimulatorRegistry
	routers            providerpolicy.RouterRegistry
	secretManagers     map[string]*external.ExternalSecretManagerAdapter
	stateFactories     map[string]backends.BundleFactory
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

func (b *Boundary) RegisterRuntime(runtime runtimecontracts.Runtime) error {
	return b.runtimes.Register(runtime)
}

func (b *Boundary) ResolveSimulator(simulatorID string) (simulatorcontracts.Simulator, error) {
	id := strings.TrimSpace(simulatorID)
	if id == "" {
		return nil, fmt.Errorf("simulator id is required")
	}
	return b.simulators.Get(id)
}

func (b *Boundary) RegisterSimulator(simulator simulatorcontracts.Simulator) error {
	return b.simulators.Register(simulator)
}

func (b *Boundary) ResolveRouter(routerID string) (routercontracts.Router, error) {
	id := strings.TrimSpace(routerID)
	if id == "" {
		return nil, fmt.Errorf("router id is required")
	}
	return b.routers.Get(id)
}

// ResolveSecretManager resolves a discovered secret-manager plugin adapter by ID.
func (b *Boundary) ResolveSecretManager(id string) (*external.ExternalSecretManagerAdapter, error) {
	normalized := strings.TrimSpace(id)
	if normalized == "" {
		return nil, fmt.Errorf("secret manager id is required")
	}
	adapter, ok := b.secretManagers[normalized]
	if !ok {
		return nil, fmt.Errorf("secret manager plugin %q is not registered", normalized)
	}
	return adapter, nil
}

func (b *Boundary) ResolveStateBundleFactory(kind string) (backends.BundleFactory, error) {
	normalized := strings.ToLower(strings.TrimSpace(kind))
	if normalized == "" {
		return nil, fmt.Errorf("state backend kind is required")
	}
	factory, ok := b.stateFactories[normalized]
	if !ok || factory == nil {
		return nil, fmt.Errorf("state backend kind %q is not registered", normalized)
	}
	return factory, nil
}

func (b *Boundary) RegisterStateBundleFactory(kind string, factory backends.BundleFactory) error {
	normalized := strings.ToLower(strings.TrimSpace(kind))
	if normalized == "" {
		return fmt.Errorf("state backend kind is required")
	}
	if factory == nil {
		return fmt.Errorf("state backend factory is nil")
	}
	b.stateFactories[normalized] = factory
	backends.RegisterBundleFactory(normalized, factory)
	return nil
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
		case manifests.KindRuntime:
			if _, err := b.runtimes.Get(m.ID); err == nil && !b.discoverOptions.PreferExternal && !providerpolicy.IsExternalOnlyRuntime(m.ID) {
				continue
			}
			_ = b.runtimes.Register(external.NewExternalRuntimeAdapter(m.ID, m.Executable, runtimecontracts.Meta{
				ID: m.ID, Name: m.Name, Version: m.Version, Description: m.Description,
			}))
		case manifests.KindSimulator:
			if _, err := b.simulators.Get(m.ID); err == nil && !b.discoverOptions.PreferExternal && !providerpolicy.IsExternalOnlySimulator(m.ID) {
				continue
			}
			_ = b.simulators.Register(external.NewExternalSimulatorAdapter(m.ID, m.Executable, simulatorcontracts.Meta{
				ID: m.ID, Name: m.Name, Description: m.Description,
			}))
		case manifests.KindSecretManager:
			if _, ok := b.secretManagers[m.ID]; ok && !b.discoverOptions.PreferExternal && !providerpolicy.IsExternalOnlySecretManager(m.ID) {
				continue
			}
			b.secretManagers[m.ID] = external.NewExternalSecretManagerAdapter(m.ID, m.Executable)
		case manifests.KindState:
			kind, ok := stateBackendKindFromPlugin(m.ID, m.Capabilities)
			if !ok {
				continue
			}
			if !shouldRegisterExternalStateFactory(kind, m.ID, b.discoverOptions.PreferExternal) {
				continue
			}
			_ = b.RegisterStateBundleFactory(kind, external.NewExternalStateBundleFactory(m.ID, kind, m.Executable))
		}
	}
	return nil
}

func shouldRegisterExternalStateFactory(kind, id string, preferExternal bool) bool {
	if preferExternal || providerpolicy.IsExternalOnlyState(id) {
		return true
	}
	return !isBuiltinStateBackendKind(kind)
}

func isBuiltinStateBackendKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "local", "postgres", "sqlite", "dynamodb", "s3":
		return true
	default:
		return false
	}
}

func stateBackendKindFromPlugin(pluginID string, capabilities []string) (string, bool) {
	for _, raw := range capabilities {
		capability := strings.ToLower(strings.TrimSpace(raw))
		if capability == "" {
			continue
		}
		switch {
		case strings.HasPrefix(capability, "backend:"):
			capability = strings.TrimSpace(strings.TrimPrefix(capability, "backend:"))
		case strings.HasPrefix(capability, "state:"):
			capability = strings.TrimSpace(strings.TrimPrefix(capability, "state:"))
		}
		if kind, ok := normalizeStateBackendKindToken(capability); ok {
			return kind, true
		}
	}
	return normalizeStateBackendKindToken(pluginID)
}

func normalizeStateBackendKindToken(raw string) (string, bool) {
	token := strings.ToLower(strings.TrimSpace(raw))
	if token == "" {
		return "", false
	}
	token = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, token)
	token = strings.Trim(token, "-")
	for strings.Contains(token, "--") {
		token = strings.ReplaceAll(token, "--", "-")
	}
	if strings.Contains(token, "-") {
		parts := strings.Split(token, "-")
		token = strings.TrimSpace(parts[len(parts)-1])
	}
	if token == "" {
		return "", false
	}
	return token, true
}
