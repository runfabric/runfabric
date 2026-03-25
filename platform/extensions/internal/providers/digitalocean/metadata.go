package digitalocean

import "github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"

const (
	ProviderID                     = "digitalocean-functions"
	ProviderName                   = "DigitalOcean Functions"
	ProviderDescription            = "Deploy and run functions on DigitalOcean App Platform"
	ProviderBuiltinImplementation  = false
	ProviderExcludeFromAPIDispatch = false
	ProviderIncludeBuiltinManifest = true
)

func Descriptor() catalog.ProviderDescriptor {
	return catalog.ProviderDescriptor{
		ID:                     ProviderID,
		Name:                   ProviderName,
		Description:            ProviderDescription,
		BuiltinImplementation:  ProviderBuiltinImplementation,
		ExcludeFromAPIDispatch: ProviderExcludeFromAPIDispatch,
		IncludeBuiltinManifest: ProviderIncludeBuiltinManifest,
	}
}
