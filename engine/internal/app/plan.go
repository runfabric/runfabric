package app

import "github.com/runfabric/runfabric/engine/internal/lifecycle"

func Plan(configPath, stage, providerOverride string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}
	return lifecycle.Plan(ctx.Registry, ctx.Config, ctx.Stage, ctx.RootDir)
}
