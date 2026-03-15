package app

import (
	"context"

	deployapi "github.com/runfabric/runfabric/internal/deploy/api"
	"github.com/runfabric/runfabric/internal/lifecycle"
)

func Logs(configPath, stage, function string) (any, error) {
	ctx, err := Bootstrap(configPath, stage)
	if err != nil {
		return nil, err
	}
	provider := ctx.Config.Provider.Name
	if deployapi.HasLogger(provider) {
		return deployapi.Logs(context.Background(), provider, ctx.Config, ctx.Stage, function, ctx.RootDir)
	}
	return lifecycle.Logs(ctx.Registry, ctx.Config, ctx.Stage, function)
}
