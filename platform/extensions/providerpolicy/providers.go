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
	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
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

var providerEntries = func() []catalog.ProviderPolicyEntry {
	awsHooks := inprocess.APIDispatchHooks{
		PrepareDevStream:      awsprovider.PrepareDevStreamPolicy,
		FetchMetrics:          awsprovider.FetchMetricsPolicy,
		FetchTraces:           awsprovider.FetchTracesPolicy,
		Recover:               awsprovider.RecoverPolicy,
		SyncOrchestrations:    awsprovider.SyncOrchestrationsPolicy,
		RemoveOrchestrations:  awsprovider.RemoveOrchestrationsPolicy,
		InvokeOrchestration:   awsprovider.InvokeOrchestrationPolicy,
		InspectOrchestrations: awsprovider.InspectOrchestrationsPolicy,
	}
	awsOps := inprocess.APIOps{
		Deploy: awsprovider.DeployAPIOps,
		Remove: awsprovider.RemoveAPIOps,
		Invoke: awsprovider.InvokeAPIOps,
		Logs:   awsprovider.LogsAPIOps,
	}

	gcpHooks := inprocess.APIDispatchHooks{
		PrepareDevStream:      gcpprovider.PrepareDevStreamPolicy,
		FetchMetrics:          gcpprovider.FetchMetricsPolicy,
		FetchTraces:           gcpprovider.FetchTracesPolicy,
		Recover:               gcpprovider.RecoverPolicy,
		SyncOrchestrations:    gcpprovider.SyncOrchestrationsPolicy,
		RemoveOrchestrations:  gcpprovider.RemoveOrchestrationsPolicy,
		InvokeOrchestration:   gcpprovider.InvokeOrchestrationPolicy,
		InspectOrchestrations: gcpprovider.InspectOrchestrationsPolicy,
	}
	gcpOps := inprocess.APIOps{
		Deploy: (&gcpprovider.Runner{}).Deploy,
		Remove: (&gcpprovider.Remover{}).Remove,
		Invoke: (&gcpprovider.Invoker{}).Invoke,
		Logs:   (&gcpprovider.Logger{}).Logs,
	}

	azureHooks := inprocess.APIDispatchHooks{
		PrepareDevStream:      azureprovider.PrepareDevStreamPolicy,
		FetchMetrics:          azureprovider.FetchMetricsPolicy,
		FetchTraces:           azureprovider.FetchTracesPolicy,
		Recover:               azureprovider.RecoverPolicy,
		SyncOrchestrations:    azureprovider.SyncOrchestrationsPolicy,
		RemoveOrchestrations:  azureprovider.RemoveOrchestrationsPolicy,
		InvokeOrchestration:   azureprovider.InvokeOrchestrationPolicy,
		InspectOrchestrations: azureprovider.InspectOrchestrationsPolicy,
	}
	azureOps := inprocess.APIOps{
		Deploy: (&azureprovider.Runner{}).Deploy,
		Remove: (&azureprovider.Remover{}).Remove,
		Invoke: (&azureprovider.Invoker{}).Invoke,
		Logs:   (&azureprovider.Logger{}).Logs,
	}

	alibabaHooks := inprocess.APIDispatchHooks{PrepareDevStream: alibabaprovider.PrepareDevStreamPolicy}
	alibabaOps := inprocess.APIOps{
		Deploy: (&alibabaprovider.Runner{}).Deploy,
		Remove: (&alibabaprovider.Remover{}).Remove,
		Invoke: (&alibabaprovider.Invoker{}).Invoke,
		Logs:   (&alibabaprovider.Logger{}).Logs,
	}

	cfHooks := inprocess.APIDispatchHooks{PrepareDevStream: cfprovider.PrepareDevStreamPolicy}
	cfOps := inprocess.APIOps{
		Deploy: (&cfprovider.Runner{}).Deploy,
		Remove: (&cfprovider.Remover{}).Remove,
		Invoke: (&cfprovider.Invoker{}).Invoke,
		Logs:   (&cfprovider.Logger{}).Logs,
	}

	doHooks := inprocess.APIDispatchHooks{PrepareDevStream: digitaloceanprovider.PrepareDevStreamPolicy}
	doOps := inprocess.APIOps{
		Deploy: (&digitaloceanprovider.Runner{}).Deploy,
		Remove: (&digitaloceanprovider.Remover{}).Remove,
		Invoke: (&digitaloceanprovider.Invoker{}).Invoke,
		Logs:   (&digitaloceanprovider.Logger{}).Logs,
	}

	flyHooks := inprocess.APIDispatchHooks{PrepareDevStream: flyprovider.PrepareDevStreamPolicy}
	flyOps := inprocess.APIOps{
		Deploy: (&flyprovider.Runner{}).Deploy,
		Remove: (&flyprovider.Remover{}).Remove,
		Invoke: (&flyprovider.Invoker{}).Invoke,
		Logs:   (&flyprovider.Logger{}).Logs,
	}

	ibmHooks := inprocess.APIDispatchHooks{PrepareDevStream: ibmprovider.PrepareDevStreamPolicy}
	ibmOps := inprocess.APIOps{
		Deploy: (&ibmprovider.Runner{}).Deploy,
		Remove: (&ibmprovider.Remover{}).Remove,
		Invoke: (&ibmprovider.Invoker{}).Invoke,
		Logs:   (&ibmprovider.Logger{}).Logs,
	}

	k8sHooks := inprocess.APIDispatchHooks{PrepareDevStream: kubernetesprovider.PrepareDevStreamPolicy}
	k8sOps := inprocess.APIOps{
		Deploy: (&kubernetesprovider.Runner{}).Deploy,
		Remove: (&kubernetesprovider.Remover{}).Remove,
		Invoke: (&kubernetesprovider.Invoker{}).Invoke,
		Logs:   (&kubernetesprovider.Logger{}).Logs,
	}

	netlifyHooks := inprocess.APIDispatchHooks{PrepareDevStream: netlifyprovider.PrepareDevStreamPolicy}
	netlifyOps := inprocess.APIOps{
		Deploy: (&netlifyprovider.Runner{}).Deploy,
		Remove: (&netlifyprovider.Remover{}).Remove,
		Invoke: (&netlifyprovider.Invoker{}).Invoke,
		Logs:   (&netlifyprovider.Logger{}).Logs,
	}

	vercelHooks := inprocess.APIDispatchHooks{PrepareDevStream: vercelprovider.PrepareDevStreamPolicy}
	vercelOps := inprocess.APIOps{
		Deploy: (&vercelprovider.Runner{}).Deploy,
		Remove: (&vercelprovider.Remover{}).Remove,
		Invoke: (&vercelprovider.Invoker{}).Invoke,
		Logs:   (&vercelprovider.Logger{}).Logs,
	}

	return []catalog.ProviderPolicyEntry{
		{
			Descriptor: catalog.ProviderDescriptor{ID: "aws-lambda", Name: "AWS Lambda", Description: "Deploy and run functions on AWS Lambda", IncludeBuiltinManifest: true},
			Factory: func() sdkprovider.Plugin {
				return inprocess.NewAPIOpsTransportPlugin("aws-lambda", "AWS Lambda", awsOps, awsHooks)
			},
			Hooks: &awsHooks,
			Ops:   awsOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "gcp-functions", Name: "GCP Cloud Functions", Description: "Deploy and run functions on GCP Cloud Functions Gen 2", IncludeBuiltinManifest: true},
			Factory: func() sdkprovider.Plugin {
				return inprocess.NewAPIOpsTransportPlugin("gcp-functions", "GCP Cloud Functions", gcpOps, gcpHooks)
			},
			Hooks: &gcpHooks,
			Ops:   gcpOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "azure-functions", Name: "Azure Functions", Description: "Deploy and run functions on Azure Functions", IncludeBuiltinManifest: true},
			Factory: func() sdkprovider.Plugin {
				return inprocess.NewAPIOpsTransportPlugin("azure-functions", "Azure Functions", azureOps, azureHooks)
			},
			Hooks: &azureHooks,
			Ops:   azureOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "alibaba-fc", Name: "Alibaba FC", Description: "Deploy and run functions on Alibaba Cloud Function Compute", IncludeBuiltinManifest: true},
			Factory: func() sdkprovider.Plugin {
				return inprocess.NewAPIOpsTransportPlugin("alibaba-fc", "Alibaba FC", alibabaOps, alibabaHooks)
			},
			Hooks: &alibabaHooks,
			Ops:   alibabaOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "cloudflare-workers", Name: "Cloudflare Workers", Description: "Deploy and run functions on Cloudflare Workers", IncludeBuiltinManifest: true},
			Factory: func() sdkprovider.Plugin {
				return inprocess.NewAPIOpsTransportPlugin("cloudflare-workers", "Cloudflare Workers", cfOps, cfHooks)
			},
			Hooks: &cfHooks,
			Ops:   cfOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "digitalocean-functions", Name: "DigitalOcean Functions", Description: "Deploy and run functions on DigitalOcean App Platform", IncludeBuiltinManifest: true},
			Factory: func() sdkprovider.Plugin {
				return inprocess.NewAPIOpsTransportPlugin("digitalocean-functions", "DigitalOcean Functions", doOps, doHooks)
			},
			Hooks: &doHooks,
			Ops:   doOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "fly-machines", Name: "Fly.io Machines", Description: "Deploy and run functions on Fly.io Machines", IncludeBuiltinManifest: true},
			Factory: func() sdkprovider.Plugin {
				return inprocess.NewAPIOpsTransportPlugin("fly-machines", "Fly.io Machines", flyOps, flyHooks)
			},
			Hooks: &flyHooks,
			Ops:   flyOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "ibm-openwhisk", Name: "IBM OpenWhisk", Description: "Deploy and run functions on IBM Cloud Functions (OpenWhisk)", IncludeBuiltinManifest: true},
			Factory: func() sdkprovider.Plugin {
				return inprocess.NewAPIOpsTransportPlugin("ibm-openwhisk", "IBM OpenWhisk", ibmOps, ibmHooks)
			},
			Hooks: &ibmHooks,
			Ops:   ibmOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "kubernetes", Name: "Kubernetes", Description: "Deploy and run functions on Kubernetes", IncludeBuiltinManifest: true},
			Factory: func() sdkprovider.Plugin {
				return inprocess.NewAPIOpsTransportPlugin("kubernetes", "Kubernetes", k8sOps, k8sHooks)
			},
			Hooks: &k8sHooks,
			Ops:   k8sOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "netlify", Name: "Netlify", Description: "Deploy and run functions on Netlify Functions", IncludeBuiltinManifest: true},
			Factory: func() sdkprovider.Plugin {
				return inprocess.NewAPIOpsTransportPlugin("netlify", "Netlify", netlifyOps, netlifyHooks)
			},
			Hooks: &netlifyHooks,
			Ops:   netlifyOps,
		},
		{
			Descriptor: catalog.ProviderDescriptor{ID: "vercel", Name: "Vercel", Description: "Deploy and run functions on Vercel Serverless", IncludeBuiltinManifest: true},
			Factory: func() sdkprovider.Plugin {
				return inprocess.NewAPIOpsTransportPlugin("vercel", "Vercel", vercelOps, vercelHooks)
			},
			Hooks: &vercelHooks,
			Ops:   vercelOps,
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

// NewBuiltinProvidersRegistry returns a providers registry populated with all
// built-in in-process provider implementations defined in provider policy.
func NewBuiltinProvidersRegistry() *providers.Registry {
	reg := providers.NewRegistry()
	for _, e := range providerEntries {
		if !e.Descriptor.BuiltinImplementation || e.Factory == nil {
			continue
		}
		_ = reg.Register(inprocess.New(e.Factory()))
	}
	return reg
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
