package gcp

import "github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"

const (
	ProviderID                     = "gcp-functions"
	ProviderName                   = "GCP Cloud Functions"
	ProviderDescription            = "Deploy and run functions on GCP Cloud Functions Gen 2"
	ProviderBuiltinImplementation  = true
	ProviderExcludeFromAPIDispatch = true
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
