package providerpolicy

import (
	"strings"

	"github.com/runfabric/runfabric/platform/extensions/inprocess"
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
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

var providerEntries = []catalog.ProviderPolicyEntry{
	awsprovider.PolicyEntry(),
	gcpprovider.PolicyEntry(),
	azureprovider.PolicyEntry(),
	alibabaprovider.PolicyEntry(),
	cfprovider.PolicyEntry(),
	digitaloceanprovider.PolicyEntry(),
	flyprovider.PolicyEntry(),
	ibmprovider.PolicyEntry(),
	kubernetesprovider.PolicyEntry(),
	netlifyprovider.PolicyEntry(),
	vercelprovider.PolicyEntry(),
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
