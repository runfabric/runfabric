package resolution

import (
	"github.com/runfabric/runfabric/platform/extensions/application/external"
)

func newBoundary(opts Options) (*Boundary, error) {
	builtins := NewBuiltinSet()
	b := &Boundary{
		providers:  builtins.Providers,
		runtimes:   builtins.Runtimes,
		simulators: builtins.Simulators,
		routers:    builtins.Routers,
		plugins:    builtins.Plugins,
		discoverOptions: external.DiscoverOptions{
			PreferExternal: opts.PreferExternal,
			PinnedVersions: opts.PinnedVersions,
		},
		internalProviderID: map[string]struct{}{},
		apiProviderID:      builtins.APIProviderIDs,
	}
	if opts.IncludeExternal {
		if err := b.RefreshExternal(); err != nil {
			return nil, err
		}
	}
	return b, nil
}
