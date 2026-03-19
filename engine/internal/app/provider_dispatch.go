package app

import (
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/extensions/resolution"
)

type providerDispatchMode int

const (
	dispatchPlugin providerDispatchMode = iota
	dispatchAPI
	dispatchInternal
)

type providerResolution struct {
	name     string
	provider providers.Provider
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
	} else if _, ok := p.(resolution.APIDispatchProvider); ok {
		mode = dispatchAPI
	}
	return &providerResolution{name: name, provider: p, mode: mode}, nil
}
