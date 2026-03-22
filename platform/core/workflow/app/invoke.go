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
		if workflowName, ok := parseOrchestrationTarget(function); ok {
			return orchestration.InvokeOrchestration(context.Background(), providers.OrchestrationInvokeRequest{
				Config:  ctx.Config,
				Stage:   ctx.Stage,
				Root:    ctx.RootDir,
				Name:    workflowName,
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

func parseOrchestrationTarget(function string) (string, bool) {
	prefixes := []string{"sfn:", "stepfunction:", "cwf:", "cloudworkflow:", "durable:"}
	trimmed := strings.TrimSpace(function)
	for _, prefix := range prefixes {
		if strings.HasPrefix(trimmed, prefix) {
			name := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
			if name == "" {
				return "", false
			}
			return name, true
		}
	}
	return "", false
}
