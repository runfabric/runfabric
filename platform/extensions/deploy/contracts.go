package deploy

import (
	"context"
	"fmt"

	sdkbridge "github.com/runfabric/runfabric/internal/provider/sdkbridge"
	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
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
	SyncOrchestrations(ctx context.Context, req sdkprovider.OrchestrationSyncRequest) (*sdkprovider.OrchestrationSyncResult, error)
	RemoveOrchestrations(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest) (*sdkprovider.OrchestrationSyncResult, error)
	InvokeOrchestration(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest) (*sdkprovider.InvokeResult, error)
	InspectOrchestrations(ctx context.Context, req sdkprovider.OrchestrationInspectRequest) (map[string]any, error)
}

type Runner interface {
	Deploy(ctx context.Context, cfg sdkprovider.Config, stage, root string) (*sdkprovider.DeployResult, error)
}

type Remover interface {
	Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error)
}

type Invoker interface {
	Invoke(ctx context.Context, cfg sdkprovider.Config, stage, function string, payload []byte, receipt any) (*sdkprovider.InvokeResult, error)
}

type Logger interface {
	Logs(ctx context.Context, cfg sdkprovider.Config, stage, function string, receipt any) (*sdkprovider.LogsResult, error)
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
	tc, err := sdkbridge.FromCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	r, err := p.runner.Deploy(ctx, tc, stage, root)
	if err != nil {
		return nil, err
	}
	return sdkDeployToCore(r), nil
}

func (p *apiProvider) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	tc, err := sdkbridge.FromCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	r, err := p.remover.Remove(ctx, tc, stage, root, receipt)
	if err != nil {
		return nil, err
	}
	return sdkRemoveToCore(r), nil
}

func (p *apiProvider) Invoke(ctx context.Context, cfg *config.Config, stage, function string, payload []byte, receipt *state.Receipt) (*providers.InvokeResult, error) {
	tc, err := sdkbridge.FromCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	r, err := p.invoker.Invoke(ctx, tc, stage, function, payload, receipt)
	if err != nil {
		return nil, err
	}
	return sdkInvokeToCore(r), nil
}

func (p *apiProvider) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	tc, err := sdkbridge.FromCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	r, err := p.logger.Logs(ctx, tc, stage, function, receipt)
	if err != nil {
		return nil, err
	}
	return sdkLogsToCore(r), nil
}

func (p *apiProvider) SyncOrchestrations(ctx context.Context, req providers.OrchestrationSyncRequest) (*providers.OrchestrationSyncResult, error) {
	if p.orch == nil {
		return &providers.OrchestrationSyncResult{}, nil
	}
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := p.orch.SyncOrchestrations(ctx, sdkprovider.OrchestrationSyncRequest{
		Config: tc, Stage: req.Stage, Root: req.Root, FunctionResourceByName: req.FunctionResourceByName,
	})
	if err != nil {
		return nil, err
	}
	if r == nil {
		return &providers.OrchestrationSyncResult{}, nil
	}
	return &providers.OrchestrationSyncResult{Metadata: r.Metadata, Outputs: r.Outputs}, nil
}

func (p *apiProvider) RemoveOrchestrations(ctx context.Context, req providers.OrchestrationRemoveRequest) (*providers.OrchestrationSyncResult, error) {
	if p.orch == nil {
		return &providers.OrchestrationSyncResult{}, nil
	}
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := p.orch.RemoveOrchestrations(ctx, sdkprovider.OrchestrationRemoveRequest{
		Config: tc, Stage: req.Stage, Root: req.Root,
	})
	if err != nil {
		return nil, err
	}
	if r == nil {
		return &providers.OrchestrationSyncResult{}, nil
	}
	return &providers.OrchestrationSyncResult{Metadata: r.Metadata, Outputs: r.Outputs}, nil
}

func (p *apiProvider) InvokeOrchestration(ctx context.Context, req providers.OrchestrationInvokeRequest) (*providers.InvokeResult, error) {
	if p.orch == nil {
		return nil, fmt.Errorf("provider %q does not support orchestration", p.name)
	}
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := p.orch.InvokeOrchestration(ctx, sdkprovider.OrchestrationInvokeRequest{
		Config: tc, Stage: req.Stage, Root: req.Root, Name: req.Name, Payload: req.Payload,
	})
	if err != nil {
		return nil, err
	}
	return sdkInvokeToCore(r), nil
}

func (p *apiProvider) InspectOrchestrations(ctx context.Context, req providers.OrchestrationInspectRequest) (map[string]any, error) {
	if p.orch == nil {
		return map[string]any{}, nil
	}
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	return p.orch.InspectOrchestrations(ctx, sdkprovider.OrchestrationInspectRequest{Config: tc, Stage: req.Stage, Root: req.Root})
}

// opsOrchCapable wraps inprocess.APIDispatchHooks as OrchestrationCapable.
type opsOrchCapable struct{ h *inprocess.APIDispatchHooks }

func (o *opsOrchCapable) SyncOrchestrations(ctx context.Context, req sdkprovider.OrchestrationSyncRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	return o.h.SyncOrchestrations(ctx, req)
}
func (o *opsOrchCapable) RemoveOrchestrations(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	return o.h.RemoveOrchestrations(ctx, req)
}
func (o *opsOrchCapable) InvokeOrchestration(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest) (*sdkprovider.InvokeResult, error) {
	return o.h.InvokeOrchestration(ctx, req)
}
func (o *opsOrchCapable) InspectOrchestrations(ctx context.Context, req sdkprovider.OrchestrationInspectRequest) (map[string]any, error) {
	return o.h.InspectOrchestrations(ctx, req)
}

// opsRunnerAdapter adapts an APIOps.Deploy func to the Runner interface.
type opsRunnerAdapter struct {
	fn func(context.Context, sdkprovider.Config, string, string) (*sdkprovider.DeployResult, error)
}

func (a opsRunnerAdapter) Deploy(ctx context.Context, cfg sdkprovider.Config, stage, root string) (*sdkprovider.DeployResult, error) {
	return a.fn(ctx, cfg, stage, root)
}

// opsRemoverAdapter adapts an APIOps.Remove func to the Remover interface.
type opsRemoverAdapter struct {
	fn func(context.Context, sdkprovider.Config, string, string, any) (*sdkprovider.RemoveResult, error)
}

func (a opsRemoverAdapter) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	return a.fn(ctx, cfg, stage, root, receipt)
}

// opsInvokerAdapter adapts an APIOps.Invoke func to the Invoker interface.
type opsInvokerAdapter struct {
	fn func(context.Context, sdkprovider.Config, string, string, []byte, any) (*sdkprovider.InvokeResult, error)
}

func (a opsInvokerAdapter) Invoke(ctx context.Context, cfg sdkprovider.Config, stage, function string, payload []byte, receipt any) (*sdkprovider.InvokeResult, error) {
	return a.fn(ctx, cfg, stage, function, payload, receipt)
}

// opsLoggerAdapter adapts an APIOps.Logs func to the Logger interface.
type opsLoggerAdapter struct {
	fn func(context.Context, sdkprovider.Config, string, string, any) (*sdkprovider.LogsResult, error)
}

func (a opsLoggerAdapter) Logs(ctx context.Context, cfg sdkprovider.Config, stage, function string, receipt any) (*sdkprovider.LogsResult, error) {
	return a.fn(ctx, cfg, stage, function, receipt)
}

// newAPIProviderFromOps builds a Provider from APIOps and optional hooks (used for orch capability).
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

func sdkDeployToCore(r *sdkprovider.DeployResult) *providers.DeployResult {
	if r == nil {
		return nil
	}
	out := &providers.DeployResult{
		Provider:     r.Provider,
		DeploymentID: r.DeploymentID,
		Outputs:      r.Outputs,
		Metadata:     r.Metadata,
	}
	if len(r.Artifacts) > 0 {
		out.Artifacts = make([]providers.Artifact, len(r.Artifacts))
		for i, a := range r.Artifacts {
			out.Artifacts[i] = providers.Artifact{
				Function:        a.Function,
				Runtime:         a.Runtime,
				SourcePath:      a.SourcePath,
				OutputPath:      a.OutputPath,
				SHA256:          a.SHA256,
				SizeBytes:       a.SizeBytes,
				ConfigSignature: a.ConfigSignature,
			}
		}
	}
	if len(r.Functions) > 0 {
		out.Functions = make(map[string]providers.DeployedFunction, len(r.Functions))
		for k, f := range r.Functions {
			out.Functions[k] = providers.DeployedFunction{
				ResourceName:       f.ResourceName,
				ResourceIdentifier: f.ResourceIdentifier,
				Metadata:           f.Metadata,
			}
		}
	}
	return out
}

func sdkRemoveToCore(r *sdkprovider.RemoveResult) *providers.RemoveResult {
	if r == nil {
		return nil
	}
	return &providers.RemoveResult{Provider: r.Provider, Removed: r.Removed}
}

func sdkInvokeToCore(r *sdkprovider.InvokeResult) *providers.InvokeResult {
	if r == nil {
		return nil
	}
	return &providers.InvokeResult{
		Provider: r.Provider, Function: r.Function, Output: r.Output,
		RunID: r.RunID, Workflow: r.Workflow,
	}
}

func sdkLogsToCore(r *sdkprovider.LogsResult) *providers.LogsResult {
	if r == nil {
		return nil
	}
	return &providers.LogsResult{
		Provider: r.Provider, Function: r.Function, Lines: r.Lines, Workflow: r.Workflow,
	}
}
