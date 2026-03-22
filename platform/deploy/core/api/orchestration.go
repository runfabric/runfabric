package api

import (
	"context"
	"fmt"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

type orchestrationProvider interface {
	SyncOrchestrations(ctx context.Context, req providers.OrchestrationSyncRequest) (*providers.OrchestrationSyncResult, error)
	RemoveOrchestrations(ctx context.Context, req providers.OrchestrationRemoveRequest) (*providers.OrchestrationSyncResult, error)
	InvokeOrchestration(ctx context.Context, req providers.OrchestrationInvokeRequest) (*providers.InvokeResult, error)
	InspectOrchestrations(ctx context.Context, req providers.OrchestrationInspectRequest) (map[string]any, error)
}

func SyncOrchestrations(ctx context.Context, provider string, req providers.OrchestrationSyncRequest) (*providers.OrchestrationSyncResult, error) {
	p, ok := getProvider(provider)
	if !ok {
		return nil, fmt.Errorf("orchestration sync via API is not supported for unregistered provider %q", provider)
	}
	orch, ok := p.(orchestrationProvider)
	if !ok {
		return &providers.OrchestrationSyncResult{}, nil
	}
	return orch.SyncOrchestrations(ctx, req)
}

func RemoveOrchestrations(ctx context.Context, provider string, req providers.OrchestrationRemoveRequest) (*providers.OrchestrationSyncResult, error) {
	p, ok := getProvider(provider)
	if !ok {
		return nil, fmt.Errorf("orchestration remove via API is not supported for unregistered provider %q", provider)
	}
	orch, ok := p.(orchestrationProvider)
	if !ok {
		return &providers.OrchestrationSyncResult{}, nil
	}
	return orch.RemoveOrchestrations(ctx, req)
}

func InvokeOrchestration(ctx context.Context, provider string, cfg *config.Config, stage, root, name string, payload []byte) (*providers.InvokeResult, error) {
	p, ok := getProvider(provider)
	if !ok {
		return nil, fmt.Errorf("orchestration invoke via API is not supported for unregistered provider %q", provider)
	}
	orch, ok := p.(orchestrationProvider)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support orchestration", provider)
	}
	return orch.InvokeOrchestration(ctx, providers.OrchestrationInvokeRequest{
		Config:  cfg,
		Stage:   stage,
		Root:    root,
		Name:    name,
		Payload: payload,
	})
}

func InspectOrchestrations(ctx context.Context, provider string, cfg *config.Config, stage, root string) (map[string]any, error) {
	p, ok := getProvider(provider)
	if !ok {
		return nil, fmt.Errorf("orchestration inspect via API is not supported for unregistered provider %q", provider)
	}
	orch, ok := p.(orchestrationProvider)
	if !ok {
		return map[string]any{}, nil
	}
	return orch.InspectOrchestrations(ctx, providers.OrchestrationInspectRequest{Config: cfg, Stage: stage, Root: root})
}
