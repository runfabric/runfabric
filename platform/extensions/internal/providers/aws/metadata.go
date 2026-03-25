package aws

import "github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"

const (
	ProviderID                     = "aws-lambda"
	ProviderName                   = "AWS Lambda"
	ProviderDescription            = "Deploy and run functions on AWS Lambda"
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
