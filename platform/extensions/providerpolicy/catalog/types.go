package catalog

import (
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// ProviderDescriptor defines how a provider should be surfaced by the extension system.
type ProviderDescriptor struct {
	ID string

	Name        string
	Description string

	BuiltinImplementation  bool
	ExcludeFromAPIDispatch bool
	IncludeBuiltinManifest bool
}

// ProviderPolicyEntry is the provider policy record consumed by the registry layer.
type ProviderPolicyEntry struct {
	Descriptor ProviderDescriptor
	Factory    func() sdkprovider.Plugin
	Hooks      *inprocess.APIDispatchHooks
	Ops        inprocess.APIOps
}
