package app

import (
	"fmt"
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
)

type providerDispatchMode int

const (
	dispatchPlugin providerDispatchMode = iota
	dispatchAPI
	dispatchInternal
)

type providerResolution struct {
	name     string
	provider providers.ProviderPlugin
	mode     providerDispatchMode
}

func resolveProvider(ctx *AppContext) (*providerResolution, error) {
	name := strings.TrimSpace(ctx.Config.Provider.Name)
	if name == "" {
		return nil, fmt.Errorf("provider.name is required")
	}
	p, err := ctx.Extensions.ResolveProvider(name)
	if err != nil {
		return nil, err
	}
	mode := dispatchPlugin
	if p.IsInternal {
		mode = dispatchInternal
	} else if p.IsAPIDispatch {
		mode = dispatchAPI
	}
	return &providerResolution{name: name, provider: p.Provider, mode: mode}, nil
}
