package deploy

import (
	"context"
	"fmt"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// Provider is the unified internal API-dispatch interface used by deploy/core/api.
type Provider interface {
	Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error)
	Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error)
	Invoke(ctx context.Context, cfg *config.Config, stage, function string, payload []byte, receipt *state.Receipt) (*providers.InvokeResult, error)
	Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error)
}

// OrchestrationCapable is an optional API-dispatch capability for provider-native workflow engines.
type OrchestrationCapable interface {
	SyncOrchestrations(ctx context.Context, req providers.OrchestrationSyncRequest) (*providers.OrchestrationSyncResult, error)
	RemoveOrchestrations(ctx context.Context, req providers.OrchestrationRemoveRequest) (*providers.OrchestrationSyncResult, error)
	InvokeOrchestration(ctx context.Context, req providers.OrchestrationInvokeRequest) (*providers.InvokeResult, error)
	InspectOrchestrations(ctx context.Context, req providers.OrchestrationInspectRequest) (map[string]any, error)
}

type Runner interface {
	Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error)
}

type Remover interface {
	Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error)
}

type Invoker interface {
	Invoke(ctx context.Context, cfg *config.Config, stage, function string, payload []byte, receipt *state.Receipt) (*providers.InvokeResult, error)
}

type Logger interface {
	Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error)
}

type apiProvider struct {
	name    string
	runner  Runner
	remover Remover
	invoker Invoker
	logger  Logger
	orch    OrchestrationCapable
}

func (p *apiProvider) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	return p.runner.Deploy(ctx, cfg, stage, root)
}

func (p *apiProvider) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	return p.remover.Remove(ctx, cfg, stage, root, receipt)
}

func (p *apiProvider) Invoke(ctx context.Context, cfg *config.Config, stage, function string, payload []byte, receipt *state.Receipt) (*providers.InvokeResult, error) {
	return p.invoker.Invoke(ctx, cfg, stage, function, payload, receipt)
}

func (p *apiProvider) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	return p.logger.Logs(ctx, cfg, stage, function, receipt)
}

func (p *apiProvider) SyncOrchestrations(ctx context.Context, req providers.OrchestrationSyncRequest) (*providers.OrchestrationSyncResult, error) {
	if p.orch == nil {
		return &providers.OrchestrationSyncResult{}, nil
	}
	return p.orch.SyncOrchestrations(ctx, req)
}

func (p *apiProvider) RemoveOrchestrations(ctx context.Context, req providers.OrchestrationRemoveRequest) (*providers.OrchestrationSyncResult, error) {
	if p.orch == nil {
		return &providers.OrchestrationSyncResult{}, nil
	}
	return p.orch.RemoveOrchestrations(ctx, req)
}

func (p *apiProvider) InvokeOrchestration(ctx context.Context, req providers.OrchestrationInvokeRequest) (*providers.InvokeResult, error) {
	if p.orch == nil {
		return nil, fmt.Errorf("provider %q does not support orchestration", p.name)
	}
	return p.orch.InvokeOrchestration(ctx, req)
}

func (p *apiProvider) InspectOrchestrations(ctx context.Context, req providers.OrchestrationInspectRequest) (map[string]any, error) {
	if p.orch == nil {
		return map[string]any{}, nil
	}
	return p.orch.InspectOrchestrations(ctx, req)
}

func newAPIProvider(name string, runner Runner, remover Remover, invoker Invoker, logger Logger, orchestration ...OrchestrationCapable) Provider {
	if runner == nil || remover == nil || invoker == nil || logger == nil {
		panic(fmt.Sprintf("api provider %q missing required capability (deploy/remove/invoke/logs)", name))
	}
	var orch OrchestrationCapable
	if len(orchestration) > 0 {
		orch = orchestration[0]
	}
	return &apiProvider{name: name, runner: runner, remover: remover, invoker: invoker, logger: logger, orch: orch}
}
