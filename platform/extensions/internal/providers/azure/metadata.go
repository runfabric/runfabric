package azure

import "github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"

const (
	ProviderID                     = "azure-functions"
	ProviderName                   = "Azure Functions"
	ProviderDescription            = "Deploy and run functions on Azure Functions"
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
