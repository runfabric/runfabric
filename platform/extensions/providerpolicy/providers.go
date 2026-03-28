package providerpolicy

import (
	"strings"

	alibabaprovider "github.com/runfabric/runfabric/extensions/providers/alibaba"
	awsprovider "github.com/runfabric/runfabric/extensions/providers/aws"
	azureprovider "github.com/runfabric/runfabric/extensions/providers/azure"
	cfprovider "github.com/runfabric/runfabric/extensions/providers/cloudflare"
	digitaloceanprovider "github.com/runfabric/runfabric/extensions/providers/digitalocean"
	flyprovider "github.com/runfabric/runfabric/extensions/providers/fly"
	gcpprovider "github.com/runfabric/runfabric/extensions/providers/gcp"
	ibmprovider "github.com/runfabric/runfabric/extensions/providers/ibm"
	kubernetesprovider "github.com/runfabric/runfabric/extensions/providers/kubernetes"
	netlifyprovider "github.com/runfabric/runfabric/extensions/providers/netlify"
	vercelprovider "github.com/runfabric/runfabric/extensions/providers/vercel"
	builtinrouters "github.com/runfabric/runfabric/extensions/routers"
	builtinruntimes "github.com/runfabric/runfabric/extensions/runtimes"
	builtinsimulators "github.com/runfabric/runfabric/extensions/simulators"
	providers "github.com/runfabric/runfabric/internal/provider/contracts"
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
}

// SimulatorRegistry is the simulator plugin lookup surface exposed outside this package.
// providerpolicy remains the only package importing root extensions simulators.
type SimulatorRegistry interface {
	Get(simulatorID string) (simulatorcontracts.Simulator, error)
}

// RouterRegistry is the router plugin lookup/registration surface exposed outside this package.
// providerpolicy remains the only package importing root extensions routers.
type RouterRegistry interface {
	Get(routerID string) (routercontracts.Router, error)
	Register(router routercontracts.Router) error
}

var providerEntries = func() []catalog.ProviderPolicyEntry {
	awsHooks := inprocess.APIDispatchHooks{
		PrepareDevStream:      adaptPrepareDevStream(awsprovider.PrepareDevStreamPolicy),
		FetchMetrics:          adaptFetchMetrics(awsprovider.FetchMetricsPolicy),
		FetchTraces:           adaptFetchTraces(awsprovider.FetchTracesPolicy),
		Recover:               adaptRecover(awsprovider.RecoverPolicy),
		SyncOrchestrations:    adaptSyncOrchestrations(awsprovider.SyncOrchestrationsPolicy),
		RemoveOrchestrations:  adaptRemoveOrchestrations(awsprovider.RemoveOrchestrationsPolicy),
		InvokeOrchestration:   adaptInvokeOrchestration(awsprovider.InvokeOrchestrationPolicy),
		InspectOrchestrations: adaptInspectOrchestrations(awsprovider.InspectOrchestrationsPolicy),
	}
	awsOps := inprocess.APIOps{
		Deploy: adaptDeploy(awsprovider.DeployAPIOps),
		Remove: adaptRemove(awsprovider.RemoveAPIOps),
		Invoke: adaptInvoke(awsprovider.InvokeAPIOps),
		Logs:   adaptLogs(awsprovider.LogsAPIOps),
	}

	gcpHooks := inprocess.APIDispatchHooks{
		PrepareDevStream:      adaptPrepareDevStream(gcpprovider.PrepareDevStreamPolicy),
		FetchMetrics:          adaptFetchMetrics(gcpprovider.FetchMetricsPolicy),
		FetchTraces:           adaptFetchTraces(gcpprovider.FetchTracesPolicy),
		Recover:               adaptRecover(gcpprovider.RecoverPolicy),
		SyncOrchestrations:    adaptSyncOrchestrations(gcpprovider.SyncOrchestrationsPolicy),
		RemoveOrchestrations:  adaptRemoveOrchestrations(gcpprovider.RemoveOrchestrationsPolicy),
		InvokeOrchestration:   adaptInvokeOrchestration(gcpprovider.InvokeOrchestrationPolicy),
		InspectOrchestrations: adaptInspectOrchestrations(gcpprovider.InspectOrchestrationsPolicy),
	}
	gcpOps := inprocess.APIOps{
		Deploy: adaptDeploy((&gcpprovider.Runner{}).Deploy),
		Remove: adaptRemove((&gcpprovider.Remover{}).Remove),
		Invoke: adaptInvoke((&gcpprovider.Invoker{}).Invoke),
		Logs:   adaptLogs((&gcpprovider.Logger{}).Logs),
	}

	azureHooks := inprocess.APIDispatchHooks{
		PrepareDevStream:      adaptPrepareDevStream(azureprovider.PrepareDevStreamPolicy),
		FetchMetrics:          adaptFetchMetrics(azureprovider.FetchMetricsPolicy),
		FetchTraces:           adaptFetchTraces(azureprovider.FetchTracesPolicy),
		Recover:               adaptRecover(azureprovider.RecoverPolicy),
		SyncOrchestrations:    adaptSyncOrchestrations(azureprovider.SyncOrchestrationsPolicy),
		RemoveOrchestrations:  adaptRemoveOrchestrations(azureprovider.RemoveOrchestrationsPolicy),
		InvokeOrchestration:   adaptInvokeOrchestration(azureprovider.InvokeOrchestrationPolicy),
		InspectOrchestrations: adaptInspectOrchestrations(azureprovider.InspectOrchestrationsPolicy),
	}
	azureOps := inprocess.APIOps{
		Deploy: adaptDeploy((&azureprovider.Runner{}).Deploy),
		Remove: adaptRemove((&azureprovider.Remover{}).Remove),
		Invoke: adaptInvoke((&azureprovider.Invoker{}).Invoke),
		Logs:   adaptLogs((&azureprovider.Logger{}).Logs),
	}

	alibabaHooks := inprocess.APIDispatchHooks{PrepareDevStream: adaptPrepareDevStream(alibabaprovider.PrepareDevStreamPolicy)}
	alibabaOps := inprocess.APIOps{
		Deploy: adaptDeploy((&alibabaprovider.Runner{}).Deploy),
		Remove: adaptRemove((&alibabaprovider.Remover{}).Remove),
		Invoke: adaptInvoke((&alibabaprovider.Invoker{}).Invoke),
		Logs:   adaptLogs((&alibabaprovider.Logger{}).Logs),
	}

	cfHooks := inprocess.APIDispatchHooks{PrepareDevStream: adaptPrepareDevStream(cfprovider.PrepareDevStreamPolicy)}
	cfOps := inprocess.APIOps{
		Deploy: adaptDeploy((&cfprovider.Runner{}).Deploy),
		Remove: adaptRemove((&cfprovider.Remover{}).Remove),
		Invoke: adaptInvoke((&cfprovider.Invoker{}).Invoke),
		Logs:   adaptLogs((&cfprovider.Logger{}).Logs),
	}

	doHooks := inprocess.APIDispatchHooks{PrepareDevStream: adaptPrepareDevStream(digitaloceanprovider.PrepareDevStreamPolicy)}
	doOps := inprocess.APIOps{
		Deploy: adaptDeploy((&digitaloceanprovider.Runner{}).Deploy),
		Remove: adaptRemove((&digitaloceanprovider.Remover{}).Remove),
		Invoke: adaptInvoke((&digitaloceanprovider.Invoker{}).Invoke),
		Logs:   adaptLogs((&digitaloceanprovider.Logger{}).Logs),
	}

	flyHooks := inprocess.APIDispatchHooks{PrepareDevStream: adaptPrepareDevStream(flyprovider.PrepareDevStreamPolicy)}
	flyOps := inprocess.APIOps{
		Deploy: adaptDeploy((&flyprovider.Runner{}).Deploy),
		Remove: adaptRemove((&flyprovider.Remover{}).Remove),
		Invoke: adaptInvoke((&flyprovider.Invoker{}).Invoke),
		Logs:   adaptLogs((&flyprovider.Logger{}).Logs),
	}

	ibmHooks := inprocess.APIDispatchHooks{PrepareDevStream: adaptPrepareDevStream(ibmprovider.PrepareDevStreamPolicy)}
	ibmOps := inprocess.APIOps{
		Deploy: adaptDeploy((&ibmprovider.Runner{}).Deploy),
		Remove: adaptRemove((&ibmprovider.Remover{}).Remove),
		Invoke: adaptInvoke((&ibmprovider.Invoker{}).Invoke),
		Logs:   adaptLogs((&ibmprovider.Logger{}).Logs),
	}

	k8sHooks := inprocess.APIDispatchHooks{PrepareDevStream: adaptPrepareDevStream(kubernetesprovider.PrepareDevStreamPolicy)}
	k8sOps := inprocess.APIOps{
		Deploy: adaptDeploy((&kubernetesprovider.Runner{}).Deploy),
		Remove: adaptRemove((&kubernetesprovider.Remover{}).Remove),
		Invoke: adaptInvoke((&kubernetesprovider.Invoker{}).Invoke),
		Logs:   adaptLogs((&kubernetesprovider.Logger{}).Logs),
	}

	netlifyHooks := inprocess.APIDispatchHooks{PrepareDevStream: adaptPrepareDevStream(netlifyprovider.PrepareDevStreamPolicy)}
	netlifyOps := inprocess.APIOps{
		Deploy: adaptDeploy((&netlifyprovider.Runner{}).Deploy),
		Remove: adaptRemove((&netlifyprovider.Remover{}).Remove),
		Invoke: adaptInvoke((&netlifyprovider.Invoker{}).Invoke),
		Logs:   adaptLogs((&netlifyprovider.Logger{}).Logs),
	}

	vercelHooks := inprocess.APIDispatchHooks{PrepareDevStream: adaptPrepareDevStream(vercelprovider.PrepareDevStreamPolicy)}
	vercelOps := inprocess.APIOps{
		Deploy: adaptDeploy((&vercelprovider.Runner{}).Deploy),
		Remove: adaptRemove((&vercelprovider.Remover{}).Remove),
		Invoke: adaptInvoke((&vercelprovider.Invoker{}).Invoke),
		Logs:   adaptLogs((&vercelprovider.Logger{}).Logs),
	}

	return []catalog.ProviderPolicyEntry{
		{
			Descriptor: catalog.ProviderDescriptor{ID: "aws-lambda", Name: "AWS Lambda", Description: "Deploy and run functions on AWS Lambda", IncludeBuiltinManifest: true},
			Hooks:      &awsHooks,
			Ops:        awsOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "gcp-functions", Name: "GCP Cloud Functions", Description: "Deploy and run functions on GCP Cloud Functions Gen 2", IncludeBuiltinManifest: true},
			Hooks:      &gcpHooks,
			Ops:        gcpOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "azure-functions", Name: "Azure Functions", Description: "Deploy and run functions on Azure Functions", IncludeBuiltinManifest: true},
			Hooks:      &azureHooks,
			Ops:        azureOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "alibaba-fc", Name: "Alibaba FC", Description: "Deploy and run functions on Alibaba Cloud Function Compute", IncludeBuiltinManifest: true},
			Hooks:      &alibabaHooks,
			Ops:        alibabaOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "cloudflare-workers", Name: "Cloudflare Workers", Description: "Deploy and run functions on Cloudflare Workers", IncludeBuiltinManifest: true},
			Hooks:      &cfHooks,
			Ops:        cfOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "digitalocean-functions", Name: "DigitalOcean Functions", Description: "Deploy and run functions on DigitalOcean App Platform", IncludeBuiltinManifest: true},
			Hooks:      &doHooks,
			Ops:        doOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "fly-machines", Name: "Fly.io Machines", Description: "Deploy and run functions on Fly.io Machines", IncludeBuiltinManifest: true},
			Hooks:      &flyHooks,
			Ops:        flyOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "ibm-openwhisk", Name: "IBM OpenWhisk", Description: "Deploy and run functions on IBM Cloud Functions (OpenWhisk)", IncludeBuiltinManifest: true},
			Hooks:      &ibmHooks,
			Ops:        ibmOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "kubernetes", Name: "Kubernetes", Description: "Deploy and run functions on Kubernetes", IncludeBuiltinManifest: true},
			Hooks:      &k8sHooks,
			Ops:        k8sOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "netlify", Name: "Netlify", Description: "Deploy and run functions on Netlify Functions", IncludeBuiltinManifest: true},
			Hooks:      &netlifyHooks,
			Ops:        netlifyOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "vercel", Name: "Vercel", Description: "Deploy and run functions on Vercel Serverless", IncludeBuiltinManifest: true},
			Hooks:      &vercelHooks,
			Ops:        vercelOps,
		},
	}
}()

// GetAPIHooks returns the hook set for the given provider ID, or nil if the provider
// has no registered hooks (generic adapter fallbacks will be used).
func GetAPIHooks(id string) *inprocess.APIDispatchHooks {
	lookupID := strings.TrimSpace(id)
	for _, e := range providerEntries {
		if e.Descriptor.ID == lookupID {
			return e.Hooks
		}
	}
	return nil
}

func All() []catalog.ProviderDescriptor {
	out := make([]catalog.ProviderDescriptor, 0, len(providerEntries))
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

func BuiltinManifestProviders() []catalog.ProviderDescriptor {
	out := make([]catalog.ProviderDescriptor, 0, len(providerEntries))
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

// APIDispatchProviderIDs returns the IDs of all providers that participate in API dispatch.
func APIDispatchProviderIDs() []string {
	ids := make([]string, 0, len(providerEntries))
	for _, e := range providerEntries {
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
	for _, entry := range providerEntries {
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
	return toPluginMetaList(builtinruntimes.BuiltinRuntimeManifests())
}

func BuiltinSimulatorManifests() []routercontracts.PluginMeta {
	return toPluginMetaList(builtinsimulators.BuiltinSimulatorManifests())
}

func BuiltinRouterManifests() []routercontracts.PluginMeta {
	return toPluginMetaList(builtinrouters.BuiltinRouterManifests())
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
