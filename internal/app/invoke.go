package app

import (
	"context"

	deployapi "github.com/runfabric/runfabric/internal/deploy/api"
	"github.com/runfabric/runfabric/internal/lifecycle"
)

func Invoke(configPath, stage, function string, payload []byte) (any, error) {
	ctx, err := Bootstrap(configPath, stage)
	if err != nil {
		return nil, err
	}
	provider := ctx.Config.Provider.Name
	if deployapi.HasInvoker(provider) {
		return deployapi.Invoke(context.Background(), provider, ctx.Config, ctx.Stage, function, payload, ctx.RootDir)
	}
	return lifecycle.Invoke(ctx.Registry, ctx.Config, ctx.Stage, function, payload)
}
