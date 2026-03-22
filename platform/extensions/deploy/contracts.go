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

func newAPIProvider(name string, runner Runner, remover Remover, invoker Invoker, logger Logger) Provider {
	if runner == nil || remover == nil || invoker == nil || logger == nil {
		panic(fmt.Sprintf("api provider %q missing required capability (deploy/remove/invoke/logs)", name))
	}
	return &apiProvider{name: name, runner: runner, remover: remover, invoker: invoker, logger: logger}
}
