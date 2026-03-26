package app

import (
	"fmt"
	"strings"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
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
	if ctx.Extensions.IsInternalProvider(name) {
		mode = dispatchInternal
	} else if ctx.Extensions.IsAPIDispatchProvider(name) {
		mode = dispatchAPI
	}
	return &providerResolution{name: name, provider: p, mode: mode}, nil
}
