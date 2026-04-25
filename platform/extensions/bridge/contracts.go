package bridge

import (
	"context"
	"fmt"

	providercodec "github.com/runfabric/runfabric/internal/provider/codec"
	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
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
	Deploy(ctx context.Context, cfg providers.Config, stage, root string) (*providers.DeployResult, error)
}

type Remover interface {
	Remove(ctx context.Context, cfg providers.Config, stage, root string, receipt any) (*providers.RemoveResult, error)
}

type Invoker interface {
	Invoke(ctx context.Context, cfg providers.Config, stage, function string, payload []byte, receipt any) (*providers.InvokeResult, error)
}

type Logger interface {
	Logs(ctx context.Context, cfg providers.Config, stage, function string, receipt any) (*providers.LogsResult, error)
}

type apiProvider struct {
	name    string
	runner  Runner
	remover Remover
	invoker Invoker
	logger  Logger
	orch    OrchestrationCapable
}

type invalidAPIProvider struct {
	name string
	err  error
}

func (p *invalidAPIProvider) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	return nil, p.err
}

func (p *invalidAPIProvider) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	return nil, p.err
}

func (p *invalidAPIProvider) Invoke(ctx context.Context, cfg *config.Config, stage, function string, payload []byte, receipt *state.Receipt) (*providers.InvokeResult, error) {
	return nil, p.err
}

func (p *invalidAPIProvider) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	return nil, p.err
}

func (p *apiProvider) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	tc, err := providercodec.FromCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	if cs := providers.ChangesetFromContext(ctx); cs != nil {
		ctx = sdkprovider.ContextWithChangeset(ctx, convertChangeset(cs))
	}
	return p.runner.Deploy(ctx, tc, stage, root)
}

func convertChangeset(cs *providers.Changeset) *sdkprovider.Changeset {
	out := &sdkprovider.Changeset{
		Service:  cs.Service,
		Stage:    cs.Stage,
		Provider: cs.Provider,
	}
	for _, rc := range cs.Functions {
		out.Functions = append(out.Functions, sdkprovider.ResourceChange{
			Name:   rc.Name,
			Op:     sdkprovider.ChangeOp(rc.Op),
			Before: rc.Before,
			After:  rc.After,
			Reason: rc.Reason,
		})
	}
	return out
}

func (p *apiProvider) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	tc, err := providercodec.FromCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	return p.remover.Remove(ctx, tc, stage, root, receipt)
}

func (p *apiProvider) Invoke(ctx context.Context, cfg *config.Config, stage, function string, payload []byte, receipt *state.Receipt) (*providers.InvokeResult, error) {
	tc, err := providercodec.FromCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	return p.invoker.Invoke(ctx, tc, stage, function, payload, receipt)
}

func (p *apiProvider) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	tc, err := providercodec.FromCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	return p.logger.Logs(ctx, tc, stage, function, receipt)
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

// opsOrchCapable exposes APIDispatchHooks as OrchestrationCapable.
type opsOrchCapable struct{ h *inprocess.APIDispatchHooks }

func (o *opsOrchCapable) SyncOrchestrations(ctx context.Context, req providers.OrchestrationSyncRequest) (*providers.OrchestrationSyncResult, error) {
	if o == nil || o.h == nil || o.h.SyncOrchestrations == nil {
		return &providers.OrchestrationSyncResult{}, nil
	}
	return o.h.SyncOrchestrations(ctx, req)
}

func (o *opsOrchCapable) RemoveOrchestrations(ctx context.Context, req providers.OrchestrationRemoveRequest) (*providers.OrchestrationSyncResult, error) {
	if o == nil || o.h == nil || o.h.RemoveOrchestrations == nil {
		return &providers.OrchestrationSyncResult{}, nil
	}
	return o.h.RemoveOrchestrations(ctx, req)
}

func (o *opsOrchCapable) InvokeOrchestration(ctx context.Context, req providers.OrchestrationInvokeRequest) (*providers.InvokeResult, error) {
	if o == nil || o.h == nil || o.h.InvokeOrchestration == nil {
		return nil, fmt.Errorf("provider hook does not support orchestration invoke")
	}
	return o.h.InvokeOrchestration(ctx, req)
}

func (o *opsOrchCapable) InspectOrchestrations(ctx context.Context, req providers.OrchestrationInspectRequest) (map[string]any, error) {
	if o == nil || o.h == nil || o.h.InspectOrchestrations == nil {
		return map[string]any{}, nil
	}
	return o.h.InspectOrchestrations(ctx, req)
}

// opsRunnerAdapter adapts an APIOps.Deploy func to the Runner interface.
type opsRunnerAdapter struct {
	fn func(context.Context, providers.Config, string, string) (*providers.DeployResult, error)
}

func (a opsRunnerAdapter) Deploy(ctx context.Context, cfg providers.Config, stage, root string) (*providers.DeployResult, error) {
	return a.fn(ctx, cfg, stage, root)
}

// opsRemoverAdapter adapts an APIOps.Remove func to the Remover interface.
type opsRemoverAdapter struct {
	fn func(context.Context, providers.Config, string, string, any) (*providers.RemoveResult, error)
}

func (a opsRemoverAdapter) Remove(ctx context.Context, cfg providers.Config, stage, root string, receipt any) (*providers.RemoveResult, error) {
	return a.fn(ctx, cfg, stage, root, receipt)
}

// opsInvokerAdapter adapts an APIOps.Invoke func to the Invoker interface.
type opsInvokerAdapter struct {
	fn func(context.Context, providers.Config, string, string, []byte, any) (*providers.InvokeResult, error)
}

func (a opsInvokerAdapter) Invoke(ctx context.Context, cfg providers.Config, stage, function string, payload []byte, receipt any) (*providers.InvokeResult, error) {
	return a.fn(ctx, cfg, stage, function, payload, receipt)
}

// opsLoggerAdapter adapts an APIOps.Logs func to the Logger interface.
type opsLoggerAdapter struct {
	fn func(context.Context, providers.Config, string, string, any) (*providers.LogsResult, error)
}

func (a opsLoggerAdapter) Logs(ctx context.Context, cfg providers.Config, stage, function string, receipt any) (*providers.LogsResult, error) {
	return a.fn(ctx, cfg, stage, function, receipt)
}

// newAPIProviderFromOps builds a Provider from APIOps and optional hooks (used for orchestration capability).
func newAPIProviderFromOps(name string, ops inprocess.APIOps, hooks *inprocess.APIDispatchHooks) Provider {
	if ops.Deploy == nil || ops.Remove == nil || ops.Invoke == nil || ops.Logs == nil {
		return &invalidAPIProvider{
			name: name,
			err:  fmt.Errorf("api provider %q missing required capability (deploy/remove/invoke/logs)", name),
		}
	}
	var orch OrchestrationCapable
	if hooks != nil && hooks.SyncOrchestrations != nil {
		orch = &opsOrchCapable{h: hooks}
	}
	return &apiProvider{
		name:    name,
		runner:  opsRunnerAdapter{ops.Deploy},
		remover: opsRemoverAdapter{ops.Remove},
		invoker: opsInvokerAdapter{ops.Invoke},
		logger:  opsLoggerAdapter{ops.Logs},
		orch:    orch,
	}
}

func newAPIProvider(name string, runner Runner, remover Remover, invoker Invoker, logger Logger, orchestration ...OrchestrationCapable) Provider {
	if runner == nil || remover == nil || invoker == nil || logger == nil {
		return &invalidAPIProvider{
			name: name,
			err:  fmt.Errorf("api provider %q missing required capability (deploy/remove/invoke/logs)", name),
		}
	}
	var orch OrchestrationCapable
	if len(orchestration) > 0 {
		orch = orchestration[0]
	}
	return &apiProvider{name: name, runner: runner, remover: remover, invoker: invoker, logger: logger, orch: orch}
}
