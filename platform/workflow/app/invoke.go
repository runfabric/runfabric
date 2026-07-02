package app

import (
	"context"
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	deployapi "github.com/runfabric/runfabric/platform/deploy/api"
	"github.com/runfabric/runfabric/platform/workflow/lifecycle"
)

func Invoke(configPath, stage, function, providerOverride string, payload []byte) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}
	// CLI entry point: no ambient cancellation source, so use a background ctx.
	return invokeWithContext(context.Background(), ctx, function, payload)
}

// invokeWithContext dispatches an invocation on an already-bootstrapped app
// context. goCtx carries cancellation/deadline (e.g. a workflow step's
// per-step timeout) through to the real provider invoke. It is shared by Invoke
// and the workflow code-step runner.
func invokeWithContext(goCtx context.Context, ctx *AppContext, function string, payload []byte) (any, error) {
	provider, err := resolveProvider(ctx)
	if err != nil {
		return nil, err
	}

	if orchestration, ok := provider.provider.(providers.OrchestrationCapable); ok {
		if workflowName, ok := parseOrchestrationTarget(function); ok {
			return orchestration.InvokeOrchestration(goCtx, providers.OrchestrationInvokeRequest{
				Config:  ctx.Config,
				Stage:   ctx.Stage,
				Root:    ctx.RootDir,
				Name:    workflowName,
				Payload: payload,
			})
		}
	}

	if provider.mode == dispatchAPI {
		res, err := deployapi.Invoke(goCtx, provider.name, ctx.Config, ctx.Stage, function, payload, ctx.RootDir)
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
	prefixes := []string{"sfn:", "cwf:", "durable:"}
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
