package app

import (
	"github.com/runfabric/runfabric/platform/workflow/lifecycle"
)

func Plan(configPath, stage, providerOverride string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}
	planResult, err := lifecycle.Plan(ctx.Registry, ctx.Config, ctx.Stage, ctx.RootDir)
	if err != nil {
		return nil, err
	}

	return planResult, nil
}
