package netlify

import "github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"

const (
	ProviderID                     = "netlify"
	ProviderName                   = "Netlify"
	ProviderDescription            = "Deploy and run functions on Netlify Functions"
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
