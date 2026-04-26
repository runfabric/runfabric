package providers

import (
	providercontract "github.com/runfabric/runfabric/platform/core/contracts/provider"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

// LoadOptions controls provider/runtime boundary construction.
type LoadOptions struct {
	IncludeExternal bool
	PreferExternal  bool
	PinnedVersions  map[string]string
}

// LoadBoundary returns a cached engine extension boundary for the given options.
func LoadBoundary(opts LoadOptions) (*resolution.Boundary, error) {
	return resolution.NewCached(resolution.Options{
		IncludeExternal: opts.IncludeExternal,
		PreferExternal:  opts.PreferExternal,
		PinnedVersions:  opts.PinnedVersions,
	})
}

// LoadRegistry returns the provider registry resolved through the extension boundary.
func LoadRegistry(opts LoadOptions) (*providercontract.Registry, error) {
	boundary, err := LoadBoundary(opts)
	if err != nil {
		return nil, err
	}
	return boundary.ProviderRegistry(), nil
}

// ResolveProvider resolves a provider through the shared extension boundary.
func ResolveProvider(name string, opts LoadOptions) (providercontract.ProviderPlugin, error) {
	boundary, err := LoadBoundary(opts)
	if err != nil {
		return nil, err
	}
	return boundary.ResolveProvider(name)
}
