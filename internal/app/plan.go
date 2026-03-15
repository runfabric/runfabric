package app

import "github.com/runfabric/runfabric/internal/lifecycle"

func Plan(configPath, stage string) (any, error) {
	ctx, err := Bootstrap(configPath, stage)
	if err != nil {
		return nil, err
	}
	return lifecycle.Plan(ctx.Registry, ctx.Config, ctx.Stage, ctx.RootDir)
}
