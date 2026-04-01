package resolution

import (
	"github.com/runfabric/runfabric/platform/extensions/application/external"
	"github.com/runfabric/runfabric/platform/state/backends"
)

func newBoundary(opts Options) (*Boundary, error) {
	builtins := NewBuiltinSet()
	b := &Boundary{
		providers:      builtins.Providers,
		runtimes:       builtins.Runtimes,
		simulators:     builtins.Simulators,
		routers:        builtins.Routers,
		secretManagers: map[string]*external.ExternalSecretManagerAdapter{},
		stateFactories: map[string]backends.BundleFactory{},
		plugins:        builtins.Plugins,
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
