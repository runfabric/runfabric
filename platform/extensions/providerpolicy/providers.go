// Package providerpolicy is the SINGLE IMPORT GATE between platform and extensions.
//
// # Built-in vs External Plugin Control
//
// Each provider extension is wired in its own file:
//
//	builtin_aws.go          — AWS Lambda        (tag: no_builtin_aws)
//	builtin_gcp.go          — GCP Functions     (tag: no_builtin_gcp)
//	builtin_azure.go        — Azure Functions   (tag: no_builtin_azure)
//	builtin_alibaba.go      — Alibaba FC        (tag: no_builtin_alibaba)
//	builtin_cloudflare.go   — Cloudflare Workers (tag: no_builtin_cloudflare)
//	builtin_digitalocean.go — DigitalOcean      (tag: no_builtin_digitalocean)
//	builtin_fly.go          — Fly.io Machines   (tag: no_builtin_fly)
//	builtin_ibm.go          — IBM OpenWhisk     (tag: no_builtin_ibm)
//	builtin_kubernetes.go   — Kubernetes        (tag: no_builtin_kubernetes)
//	builtin_netlify.go      — Netlify           (tag: no_builtin_netlify)
//	builtin_vercel.go       — Vercel            (tag: no_builtin_vercel)
//
// To move a provider from compiled-in to external binary plugin, either:
//
//  1. Delete its builtin_<name>.go file, or
//  2. Build with -tags no_builtin_<name>  (e.g. -tags no_builtin_aws)
//
// The provider's extensions/providers/<name>/cmd/main.go binary must then be
// installed at ~/.runfabric/plugins/provider/<id>/ alongside its plugin.yaml.
//
// No other file in this package or elsewhere imports extensions/providers/*.
package providerpolicy

import (
	"sort"
	"strings"

	builtinrouters "github.com/runfabric/runfabric/extensions/routers"
	builtinruntimes "github.com/runfabric/runfabric/extensions/runtimes"
	builtinsecretmanagers "github.com/runfabric/runfabric/extensions/secretmanagers"
	builtinsimulators "github.com/runfabric/runfabric/extensions/simulators"
	builtinstates "github.com/runfabric/runfabric/extensions/states"
	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	routercontracts "github.com/runfabric/runfabric/platform/core/contracts/router"
	runtimecontracts "github.com/runfabric/runfabric/platform/core/contracts/runtime"
	simulatorcontracts "github.com/runfabric/runfabric/platform/core/contracts/simulators"
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

type APIDispatchProvider struct {
	ID    string
	Ops   inprocess.APIOps
	Hooks *inprocess.APIDispatchHooks
}

// BuiltinProviderSet is the single provider-loading view for built-in providers.
// It contains the in-process registry, builtin manifest descriptors, and the
// API-dispatch provider ID set.
type BuiltinProviderSet struct {
	Registry          *providers.Registry
	ManifestProviders []catalog.ProviderDescriptor
	APIDispatch       map[string]APIDispatchProvider
	APIProviderIDs    map[string]struct{}
}

// RuntimeRegistry is the runtime plugin lookup surface exposed outside this package.
// providerpolicy remains the only package importing root extensions runtimes.
type RuntimeRegistry interface {
	Get(runtime string) (runtimecontracts.Runtime, error)
	Register(runtime runtimecontracts.Runtime) error
}

// SimulatorRegistry is the simulator plugin lookup surface exposed outside this package.
// providerpolicy remains the only package importing root extensions simulators.
type SimulatorRegistry interface {
	Get(simulatorID string) (simulatorcontracts.Simulator, error)
	Register(simulator simulatorcontracts.Simulator) error
}

// RouterRegistry is the router plugin lookup/registration surface exposed outside this package.
// providerpolicy remains the only package importing root extensions routers.
type RouterRegistry interface {
	Get(routerID string) (routercontracts.Router, error)
	Register(router routercontracts.Router) error
}

// providerEntries is populated at init time by each builtin_<name>.go file.
// To exclude a provider from the compiled binary (and load it as an external
// plugin instead), delete its builtin_<name>.go or build with -tags no_builtin_<name>.
var providerEntries []catalog.ProviderPolicyEntry

// registerBuiltin is called from each builtin_<name>.go init() to add a provider.
func registerBuiltin(e catalog.ProviderPolicyEntry) {
	providerEntries = append(providerEntries, e)
}

var builtinProviderOrder = map[string]int{
	"aws-lambda":             10,
	"gcp-functions":          20,
	"azure-functions":        30,
	"alibaba-fc":             40,
	"cloudflare-workers":     50,
	"digitalocean-functions": 60,
	"fly-machines":           70,
	"ibm-openwhisk":          80,
	"kubernetes":             90,
	"netlify":                100,
	"vercel":                 110,
}

func orderedProviderEntries() []catalog.ProviderPolicyEntry {
	out := make([]catalog.ProviderPolicyEntry, 0, len(providerEntries))
	for _, e := range providerEntries {
		if isExternalOnlyProvider(e.Descriptor.ID) {
			continue
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		il, okI := builtinProviderOrder[out[i].Descriptor.ID]
		jl, okJ := builtinProviderOrder[out[j].Descriptor.ID]
		if okI && okJ {
			return il < jl
		}
		if okI {
			return true
		}
		if okJ {
			return false
		}
		return out[i].Descriptor.ID < out[j].Descriptor.ID
	})
	return out
}

// GetAPIHooks returns the hook set for the given provider ID, or nil if the provider
// has no registered hooks (generic adapter fallbacks will be used).
func GetAPIHooks(id string) *inprocess.APIDispatchHooks {
	lookupID := strings.TrimSpace(id)
	for _, e := range orderedProviderEntries() {
		if e.Descriptor.ID == lookupID {
			return e.Hooks
		}
	}
	return nil
}

func All() []catalog.ProviderDescriptor {
	entries := orderedProviderEntries()
	out := make([]catalog.ProviderDescriptor, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Descriptor)
	}
	return out
}

// ProviderCredentials returns the credential env vars a built-in provider
// declares (nil for unknown providers). This is the single source of truth
// consumed by the CLI doctor, .env scaffolding, and the daemon's per-request
// X-Provider-* header mapping.
func ProviderCredentials(providerID string) []catalog.CredentialVar {
	id := strings.TrimSpace(providerID)
	for _, e := range orderedProviderEntries() {
		if e.Descriptor.ID == id {
			return e.Descriptor.Credentials
		}
	}
	return nil
}

// ProviderScaffold returns the `runfabric init` scaffolding hint a built-in
// provider declares (zero value for unknown providers, meaning generic language
// defaults). This is the single source of truth for the provider-specific parts of
// project scaffolding — entry file/shape, runtime id, region env, and comment —
// consumed by the CLI init command.
func ProviderScaffold(providerID string) catalog.ProviderScaffold {
	id := strings.TrimSpace(providerID)
	for _, e := range orderedProviderEntries() {
		if e.Descriptor.ID == id {
			return e.Descriptor.Scaffold
		}
	}
	return catalog.ProviderScaffold{}
}

// StateBackendCredentials returns the credential env vars a built-in state
// backend declares (nil for unknown kinds or backends needing none, like
// local/sqlite). Vars with a Header (X-State-*) may also arrive per daemon
// request, like provider credentials.
func StateBackendCredentials(kind string) []catalog.CredentialVar {
	return toCredentialVars(builtinstates.BuiltinStateCredentials()[strings.TrimSpace(kind)])
}

// StateBackendScaffold returns the backend: config-block lines a built-in state
// backend contributes to `runfabric init` (nil for local/unknown). Single source
// of truth for the state part of scaffolding, so init.go carries no per-backend
// switch — the state-side counterpart of ProviderScaffold.
func StateBackendScaffold(kind string) []catalog.StateConfigLine {
	return toStateConfigLines(builtinStateScaffoldRaw()[strings.TrimSpace(kind)])
}

// AllStateBackendCredentials returns every built-in state backend's credential
// declaration, keyed by backend kind.
func AllStateBackendCredentials() map[string][]catalog.CredentialVar {
	out := map[string][]catalog.CredentialVar{}
	for kind, creds := range builtinstates.BuiltinStateCredentials() {
		out[kind] = toCredentialVars(creds)
	}
	return out
}

func BuiltinImplementationIDs() []string {
	entries := orderedProviderEntries()
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Descriptor.BuiltinImplementation {
			ids = append(ids, e.Descriptor.ID)
		}
	}
	return ids
}

func BuiltinManifestProviders() []catalog.ProviderDescriptor {
	entries := orderedProviderEntries()
	out := make([]catalog.ProviderDescriptor, 0, len(entries))
	for _, e := range entries {
		if e.Descriptor.IncludeBuiltinManifest {
			out = append(out, e.Descriptor)
		}
	}
	return out
}

func ExcludeFromAPIDispatch(providerID string) bool {
	id := strings.TrimSpace(providerID)
	for _, e := range orderedProviderEntries() {
		if e.Descriptor.ID == id {
			return e.Descriptor.ExcludeFromAPIDispatch
		}
	}
	return false
}

// APIDispatchProviderIDs returns the IDs of all providers that participate in API dispatch.
func APIDispatchProviderIDs() []string {
	entries := orderedProviderEntries()
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.Descriptor.ExcludeFromAPIDispatch {
			ids = append(ids, e.Descriptor.ID)
		}
	}
	return ids
}

// NewBuiltinProvidersRegistry returns the provider registry for built-ins.
func NewBuiltinProvidersRegistry() *providers.Registry {
	return providers.NewRegistry()
}

// NewBuiltinProviderSet returns the centralized built-in provider loading set.
func NewBuiltinProviderSet() *BuiltinProviderSet {
	apiDispatch := map[string]APIDispatchProvider{}
	apiProviderIDs := map[string]struct{}{}
	for _, entry := range orderedProviderEntries() {
		if entry.Descriptor.ExcludeFromAPIDispatch {
			continue
		}
		apiProviderIDs[entry.Descriptor.ID] = struct{}{}
		apiDispatch[entry.Descriptor.ID] = APIDispatchProvider{
			ID:    entry.Descriptor.ID,
			Ops:   entry.Ops,
			Hooks: entry.Hooks,
		}
	}
	return &BuiltinProviderSet{
		Registry:          NewBuiltinProvidersRegistry(),
		ManifestProviders: BuiltinManifestProviders(),
		APIDispatch:       apiDispatch,
		APIProviderIDs:    apiProviderIDs,
	}
}

func BuiltinRuntimeManifests() []routercontracts.PluginMeta {
	raw := toPluginMetaList(builtinruntimes.BuiltinRuntimeManifests())
	out := make([]routercontracts.PluginMeta, 0, len(raw))
	for _, m := range raw {
		if isExternalOnlyRuntime(m.ID) {
			continue
		}
		out = append(out, m)
	}
	return out
}

func BuiltinSimulatorManifests() []routercontracts.PluginMeta {
	raw := toPluginMetaList(builtinsimulators.BuiltinSimulatorManifests())
	out := make([]routercontracts.PluginMeta, 0, len(raw))
	for _, m := range raw {
		if isExternalOnlySimulator(m.ID) {
			continue
		}
		out = append(out, m)
	}
	return out
}

func BuiltinRouterManifests() []routercontracts.PluginMeta {
	raw := toPluginMetaList(builtinrouters.BuiltinRouterManifests())
	out := make([]routercontracts.PluginMeta, 0, len(raw))
	for _, m := range raw {
		if isExternalOnlyRouter(m.ID) {
			continue
		}
		out = append(out, m)
	}
	return out
}

func BuiltinSecretManagerManifests() []routercontracts.PluginMeta {
	raw := toPluginMetaList(builtinsecretmanagers.BuiltinSecretManagerManifests())
	out := make([]routercontracts.PluginMeta, 0, len(raw))
	for _, m := range raw {
		if isExternalOnlySecretManager(m.ID) {
			continue
		}
		out = append(out, m)
	}
	return out
}

func BuiltinStateManifests() []routercontracts.PluginMeta {
	raw := toPluginMetaList(builtinstates.BuiltinStateManifests())
	out := make([]routercontracts.PluginMeta, 0, len(raw))
	for _, m := range raw {
		if isExternalOnlyState(m.ID) {
			continue
		}
		out = append(out, m)
	}
	return out
}

func NewBuiltinRuntimeRegistry() RuntimeRegistry {
	return newRuntimeRegistry(builtinruntimes.NewBuiltinRegistry())
}

func NewBuiltinSimulatorRegistry() SimulatorRegistry {
	return newSimulatorRegistry(builtinsimulators.NewBuiltinRegistry())
}

func NewBuiltinRouterRegistry() RouterRegistry {
	return newRouterRegistry(builtinrouters.NewBuiltinRegistry())
}

func NormalizeRuntimeID(runtime string) string {
	return builtinruntimes.NormalizeRuntimeID(runtime)
}

// BackendKindFromPlugin derives the backend.kind from plugin capabilities or ID.
func BackendKindFromPlugin(pluginID string, capabilities []string) (string, bool) {
	return builtinstates.BackendKindFromPlugin(pluginID, capabilities)
}

// BackendKindFromPluginID extracts backend.kind from a state plugin ID.
func BackendKindFromPluginID(id string) (string, bool) {
	return builtinstates.BackendKindFromPluginID(id)
}

// ExpandLookupAliases normalizes additional lookup aliases for state backends.
func ExpandLookupAliases(keys map[string]struct{}) {
	builtinstates.ExpandLookupAliases(keys)
}
