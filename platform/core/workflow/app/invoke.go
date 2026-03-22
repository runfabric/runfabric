package app

import (
	"context"
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/workflow/lifecycle"
	deployapi "github.com/runfabric/runfabric/platform/deploy/core/api"
)

func Invoke(configPath, stage, function, providerOverride string, payload []byte) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}
	provider, err := resolveProvider(ctx)
	if err != nil {
		return nil, err
	}

	if orchestration, ok := provider.provider.(providers.OrchestrationCapable); ok {
		if strings.HasPrefix(function, "sfn:") || strings.HasPrefix(function, "stepfunction:") {
			name := strings.TrimPrefix(strings.TrimPrefix(function, "sfn:"), "stepfunction:")
			return orchestration.InvokeOrchestration(context.Background(), providers.OrchestrationInvokeRequest{
				Config:  ctx.Config,
				Stage:   ctx.Stage,
				Root:    ctx.RootDir,
				Name:    name,
				Payload: payload,
			})
		}
	}

	if provider.mode == dispatchAPI {
		res, err := deployapi.Invoke(context.Background(), provider.name, ctx.Config, ctx.Stage, function, payload, ctx.RootDir)
		if err != nil {
			return nil, err
		}
		return res, nil
	}
	res, err := lifecycle.Invoke(ctx.Registry, ctx.Config, ctx.Stage, function, payload)
	if err != nil {
		return nil, err
	}
	return res, nil
}
