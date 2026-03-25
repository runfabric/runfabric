package ibm

import "github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"

const (
	ProviderID                     = "ibm-openwhisk"
	ProviderName                   = "IBM OpenWhisk"
	ProviderDescription            = "Deploy and run functions on IBM Cloud Functions (OpenWhisk)"
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
