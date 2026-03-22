package deploy

import (
	"sort"

	"github.com/runfabric/runfabric/platform/extensions/internal/providers/alibaba"
	"github.com/runfabric/runfabric/platform/extensions/internal/providers/azure"
	cf "github.com/runfabric/runfabric/platform/extensions/internal/providers/cloudflare"
	do "github.com/runfabric/runfabric/platform/extensions/internal/providers/digitalocean"
	"github.com/runfabric/runfabric/platform/extensions/internal/providers/fly"
	extgcpprovider "github.com/runfabric/runfabric/platform/extensions/internal/providers/gcp"
	"github.com/runfabric/runfabric/platform/extensions/internal/providers/ibm"
	k8s "github.com/runfabric/runfabric/platform/extensions/internal/providers/kubernetes"
	"github.com/runfabric/runfabric/platform/extensions/internal/providers/netlify"
	"github.com/runfabric/runfabric/platform/extensions/internal/providers/vercel"
)

var apiProviders = map[string]Provider{
	"digitalocean-functions": newAPIProvider("digitalocean-functions", &do.Runner{}, &do.Remover{}, &do.Invoker{}, &do.Logger{}),
	"cloudflare-workers":     newAPIProvider("cloudflare-workers", &cf.Runner{}, &cf.Remover{}, &cf.Invoker{}, &cf.Logger{}),
	"vercel":                 newAPIProvider("vercel", &vercel.Runner{}, &vercel.Remover{}, &vercel.Invoker{}, &vercel.Logger{}),
	"netlify":                newAPIProvider("netlify", &netlify.Runner{}, &netlify.Remover{}, &netlify.Invoker{}, &netlify.Logger{}),
	"fly-machines":           newAPIProvider("fly-machines", &fly.Runner{}, &fly.Remover{}, &fly.Invoker{}, &fly.Logger{}),
	"gcp-functions":          newAPIProvider("gcp-functions", &extgcpprovider.Runner{}, &extgcpprovider.Remover{}, &extgcpprovider.Invoker{}, &extgcpprovider.Logger{}),
	"azure-functions":        newAPIProvider("azure-functions", &azure.Runner{}, &azure.Remover{}, &azure.Invoker{}, &azure.Logger{}, &azure.Runner{}),
	"kubernetes":             newAPIProvider("kubernetes", &k8s.Runner{}, &k8s.Remover{}, &k8s.Invoker{}, &k8s.Logger{}),
	"alibaba-fc":             newAPIProvider("alibaba-fc", &alibaba.Runner{}, &alibaba.Remover{}, &alibaba.Invoker{}, &alibaba.Logger{}),
	"ibm-openwhisk":          newAPIProvider("ibm-openwhisk", &ibm.Runner{}, &ibm.Remover{}, &ibm.Invoker{}, &ibm.Logger{}),
}

// GetProvider returns the API-dispatch provider for name, or nil, false if not found.
func GetProvider(name string) (Provider, bool) {
	p, ok := apiProviders[name]
	return p, ok
}

// HasProvider returns whether name has an API-dispatch provider.
func HasProvider(name string) bool {
	_, ok := apiProviders[name]
	return ok
}

// APIProviderNames returns the sorted list of API-dispatch provider names.
func APIProviderNames() []string {
	names := make([]string, 0, len(apiProviders))
	for k := range apiProviders {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
