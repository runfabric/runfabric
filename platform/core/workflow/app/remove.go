package app

import (
	"context"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/workflow/lifecycle"
	deployapi "github.com/runfabric/runfabric/platform/deploy/core/api"
)

func Remove(configPath, stage, providerOverride string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}

	provider, err := resolveProvider(ctx)
	if err != nil {
		return nil, err
	}

	if provider.mode == dispatchAPI {
		res, err := deployapi.Remove(context.Background(), provider.name, ctx.Config, ctx.Stage, ctx.RootDir)
		if err != nil {
			return nil, err
		}
		if orchestration, ok := provider.provider.(providers.OrchestrationCapable); ok {
			if _, err := orchestration.RemoveOrchestrations(context.Background(), providers.OrchestrationRemoveRequest{Config: ctx.Config, Stage: ctx.Stage, Root: ctx.RootDir}); err != nil {
				return nil, err
			}
		}
		return res, nil
	}

	// Internal + plugin-dispatched providers use the shared lifecycle contract.
	return lifecycle.Remove(ctx.Registry, ctx.Config, ctx.Stage, ctx.RootDir)
}
