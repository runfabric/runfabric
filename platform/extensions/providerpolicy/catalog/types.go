package catalog

import (
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
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
	Hooks      *inprocess.APIDispatchHooks
	Ops        inprocess.APIOps
}
