package providerpolicy

import (
	"context"
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	alibabaprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/alibaba"
	awsprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/aws"
	azureprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/azure"
	cfprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/cloudflare"
	digitaloceanprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/digitalocean"
	flyprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/fly"
	gcpprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/gcp"
	ibmprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/ibm"
	kubernetesprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/kubernetes"
	netlifyprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/netlify"
	vercelprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/vercel"
)

// ProviderDescriptor defines how a provider should be surfaced by the extension system.
// Update this file to change provider mode (built-in vs external, API-dispatch behavior,
// and whether it appears in built-in plugin manifests).
type ProviderDescriptor struct {
	ID string

	Name        string
	Description string

	BuiltinImplementation  bool
	ExcludeFromAPIDispatch bool
	IncludeBuiltinManifest bool
}

// ProviderAPIHooks holds optional per-provider function implementations for API-level
// operations (dev-stream, metrics, traces, recovery, orchestration). Nil fields mean the provider
// uses the generic fallback in the adapter.
type ProviderAPIHooks struct {
	PrepareDevStream      func(ctx context.Context, cfg *providers.Config, stage, tunnelURL string) (*providers.DevStreamSession, error)
	FetchMetrics          func(ctx context.Context, cfg *providers.Config, stage string) (*providers.MetricsResult, error)
	FetchTraces           func(ctx context.Context, cfg *providers.Config, stage string) (*providers.TracesResult, error)
	Recover               func(ctx context.Context, req providers.RecoveryRequest) (*providers.RecoveryResult, error)
	SyncOrchestrations    func(ctx context.Context, req providers.OrchestrationSyncRequest) (*providers.OrchestrationSyncResult, error)
	RemoveOrchestrations  func(ctx context.Context, req providers.OrchestrationRemoveRequest) (*providers.OrchestrationSyncResult, error)
	InvokeOrchestration   func(ctx context.Context, req providers.OrchestrationInvokeRequest) (*providers.InvokeResult, error)
	InspectOrchestrations func(ctx context.Context, req providers.OrchestrationInspectRequest) (map[string]any, error)
}

// ProviderPolicyEntry is the single source of truth for provider policy and optional
// provider-specific hooks used by API-dispatch adapters.
type ProviderPolicyEntry struct {
	Descriptor ProviderDescriptor
	Factory    func() providers.ProviderPlugin
	Hooks      *ProviderAPIHooks
}

var providerEntries = []ProviderPolicyEntry{
	{
		Descriptor: ProviderDescriptor{
			ID:                     awsprovider.ProviderID,
			Name:                   awsprovider.ProviderName,
			Description:            awsprovider.ProviderDescription,
			BuiltinImplementation:  awsprovider.ProviderBuiltinImplementation,
			ExcludeFromAPIDispatch: awsprovider.ProviderExcludeFromAPIDispatch,
			IncludeBuiltinManifest: awsprovider.ProviderIncludeBuiltinManifest,
		},
		Factory: func() providers.ProviderPlugin { return awsprovider.New() },
	},
	{
		Descriptor: ProviderDescriptor{
			ID:                     gcpprovider.ProviderID,
			Name:                   gcpprovider.ProviderName,
			Description:            gcpprovider.ProviderDescription,
			BuiltinImplementation:  gcpprovider.ProviderBuiltinImplementation,
			ExcludeFromAPIDispatch: gcpprovider.ProviderExcludeFromAPIDispatch,
			IncludeBuiltinManifest: gcpprovider.ProviderIncludeBuiltinManifest,
		},
		Factory: func() providers.ProviderPlugin { return gcpprovider.New() },
	},
	{
		Descriptor: ProviderDescriptor{
			ID:                     azureprovider.ProviderID,
			Name:                   azureprovider.ProviderName,
			Description:            azureprovider.ProviderDescription,
			BuiltinImplementation:  azureprovider.ProviderBuiltinImplementation,
			ExcludeFromAPIDispatch: azureprovider.ProviderExcludeFromAPIDispatch,
			IncludeBuiltinManifest: azureprovider.ProviderIncludeBuiltinManifest,
		},
		Hooks: &ProviderAPIHooks{
			PrepareDevStream:      azureprovider.PrepareDevStreamPolicy,
			FetchMetrics:          azureprovider.FetchMetricsPolicy,
			FetchTraces:           azureprovider.FetchTracesPolicy,
			Recover:               azureprovider.RecoverPolicy,
			SyncOrchestrations:    azureprovider.SyncOrchestrationsPolicy,
			RemoveOrchestrations:  azureprovider.RemoveOrchestrationsPolicy,
			InvokeOrchestration:   azureprovider.InvokeOrchestrationPolicy,
			InspectOrchestrations: azureprovider.InspectOrchestrationsPolicy,
		},
	},
	{
		Descriptor: ProviderDescriptor{
			ID:                     alibabaprovider.ProviderID,
			Name:                   alibabaprovider.ProviderName,
			Description:            alibabaprovider.ProviderDescription,
			BuiltinImplementation:  alibabaprovider.ProviderBuiltinImplementation,
			ExcludeFromAPIDispatch: alibabaprovider.ProviderExcludeFromAPIDispatch,
			IncludeBuiltinManifest: alibabaprovider.ProviderIncludeBuiltinManifest,
		},
		Hooks: &ProviderAPIHooks{
			PrepareDevStream: alibabaprovider.PrepareDevStreamPolicy,
		},
	},
	{
		Descriptor: ProviderDescriptor{
			ID:                     cfprovider.ProviderID,
			Name:                   cfprovider.ProviderName,
			Description:            cfprovider.ProviderDescription,
			BuiltinImplementation:  cfprovider.ProviderBuiltinImplementation,
			ExcludeFromAPIDispatch: cfprovider.ProviderExcludeFromAPIDispatch,
			IncludeBuiltinManifest: cfprovider.ProviderIncludeBuiltinManifest,
		},
		Hooks: &ProviderAPIHooks{
			PrepareDevStream: cfprovider.PrepareDevStreamPolicy,
		},
	},
	{
		Descriptor: ProviderDescriptor{
			ID:                     digitaloceanprovider.ProviderID,
			Name:                   digitaloceanprovider.ProviderName,
			Description:            digitaloceanprovider.ProviderDescription,
			BuiltinImplementation:  digitaloceanprovider.ProviderBuiltinImplementation,
			ExcludeFromAPIDispatch: digitaloceanprovider.ProviderExcludeFromAPIDispatch,
			IncludeBuiltinManifest: digitaloceanprovider.ProviderIncludeBuiltinManifest,
		},
		Hooks: &ProviderAPIHooks{
			PrepareDevStream: digitaloceanprovider.PrepareDevStreamPolicy,
		},
	},
	{
		Descriptor: ProviderDescriptor{
			ID:                     flyprovider.ProviderID,
			Name:                   flyprovider.ProviderName,
			Description:            flyprovider.ProviderDescription,
			BuiltinImplementation:  flyprovider.ProviderBuiltinImplementation,
			ExcludeFromAPIDispatch: flyprovider.ProviderExcludeFromAPIDispatch,
			IncludeBuiltinManifest: flyprovider.ProviderIncludeBuiltinManifest,
		},
		Hooks: &ProviderAPIHooks{
			PrepareDevStream: flyprovider.PrepareDevStreamPolicy,
		},
	},
	{
		Descriptor: ProviderDescriptor{
			ID:                     ibmprovider.ProviderID,
			Name:                   ibmprovider.ProviderName,
			Description:            ibmprovider.ProviderDescription,
			BuiltinImplementation:  ibmprovider.ProviderBuiltinImplementation,
			ExcludeFromAPIDispatch: ibmprovider.ProviderExcludeFromAPIDispatch,
			IncludeBuiltinManifest: ibmprovider.ProviderIncludeBuiltinManifest,
		},
		Hooks: &ProviderAPIHooks{
			PrepareDevStream: ibmprovider.PrepareDevStreamPolicy,
		},
	},
	{
		Descriptor: ProviderDescriptor{
			ID:                     kubernetesprovider.ProviderID,
			Name:                   kubernetesprovider.ProviderName,
			Description:            kubernetesprovider.ProviderDescription,
			BuiltinImplementation:  kubernetesprovider.ProviderBuiltinImplementation,
			ExcludeFromAPIDispatch: kubernetesprovider.ProviderExcludeFromAPIDispatch,
			IncludeBuiltinManifest: kubernetesprovider.ProviderIncludeBuiltinManifest,
		},
		Hooks: &ProviderAPIHooks{
			PrepareDevStream: kubernetesprovider.PrepareDevStreamPolicy,
		},
	},
	{
		Descriptor: ProviderDescriptor{
			ID:                     netlifyprovider.ProviderID,
			Name:                   netlifyprovider.ProviderName,
			Description:            netlifyprovider.ProviderDescription,
			BuiltinImplementation:  netlifyprovider.ProviderBuiltinImplementation,
			ExcludeFromAPIDispatch: netlifyprovider.ProviderExcludeFromAPIDispatch,
			IncludeBuiltinManifest: netlifyprovider.ProviderIncludeBuiltinManifest,
		},
		Hooks: &ProviderAPIHooks{
			PrepareDevStream: netlifyprovider.PrepareDevStreamPolicy,
		},
	},
	{
		Descriptor: ProviderDescriptor{
			ID:                     vercelprovider.ProviderID,
			Name:                   vercelprovider.ProviderName,
			Description:            vercelprovider.ProviderDescription,
			BuiltinImplementation:  vercelprovider.ProviderBuiltinImplementation,
			ExcludeFromAPIDispatch: vercelprovider.ProviderExcludeFromAPIDispatch,
			IncludeBuiltinManifest: vercelprovider.ProviderIncludeBuiltinManifest,
		},
		Hooks: &ProviderAPIHooks{
			PrepareDevStream: vercelprovider.PrepareDevStreamPolicy,
		},
	},
}

func All() []ProviderDescriptor {
	out := make([]ProviderDescriptor, 0, len(providerEntries))
	for _, e := range providerEntries {
		out = append(out, e.Descriptor)
	}
	return out
}

func BuiltinImplementationIDs() []string {
	ids := make([]string, 0, len(providerEntries))
	for _, e := range providerEntries {
		if e.Descriptor.BuiltinImplementation {
			ids = append(ids, e.Descriptor.ID)
		}
	}
	return ids
}

func BuiltinManifestProviders() []ProviderDescriptor {
	out := make([]ProviderDescriptor, 0, len(providerEntries))
	for _, e := range providerEntries {
		if e.Descriptor.IncludeBuiltinManifest {
			out = append(out, e.Descriptor)
		}
	}
	return out
}

func ExcludeFromAPIDispatch(providerID string) bool {
	id := strings.TrimSpace(providerID)
	for _, e := range providerEntries {
		if e.Descriptor.ID == id {
			return e.Descriptor.ExcludeFromAPIDispatch
		}
	}
	return false
}

// GetAPIHooks returns the hook set for the given provider ID, or nil if the provider
// has no registered hooks (generic adapter fallbacks will be used).
func GetAPIHooks(id string) *ProviderAPIHooks {
	lookupID := strings.TrimSpace(id)
	for _, e := range providerEntries {
		if e.Descriptor.ID == lookupID {
			return e.Hooks
		}
	}
	return nil
}
