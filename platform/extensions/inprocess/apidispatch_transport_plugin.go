package inprocess

import (
	"context"
	"fmt"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// APIDispatchHooks provides optional provider-specific capabilities that are not
// part of deploy/remove/invoke/logs primitives.
type APIDispatchHooks struct {
	PrepareDevStream      func(ctx context.Context, cfg sdkprovider.Config, stage, tunnelURL string) (*sdkprovider.DevStreamSession, error)
	FetchMetrics          func(ctx context.Context, cfg sdkprovider.Config, stage string) (*sdkprovider.MetricsResult, error)
	FetchTraces           func(ctx context.Context, cfg sdkprovider.Config, stage string) (*sdkprovider.TracesResult, error)
	Recover               func(ctx context.Context, req sdkprovider.RecoveryRequest) (*sdkprovider.RecoveryResult, error)
	SyncOrchestrations    func(ctx context.Context, req sdkprovider.OrchestrationSyncRequest) (*sdkprovider.OrchestrationSyncResult, error)
	RemoveOrchestrations  func(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest) (*sdkprovider.OrchestrationSyncResult, error)
	InvokeOrchestration   func(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest) (*sdkprovider.InvokeResult, error)
	InspectOrchestrations func(ctx context.Context, req sdkprovider.OrchestrationInspectRequest) (map[string]any, error)
}

// APIOps contains required API-dispatch operations.
type APIOps struct {
	Deploy func(ctx context.Context, cfg sdkprovider.Config, stage, root string) (*sdkprovider.DeployResult, error)
	Remove func(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error)
	Invoke func(ctx context.Context, cfg sdkprovider.Config, stage, function string, payload []byte, receipt any) (*sdkprovider.InvokeResult, error)
	Logs   func(ctx context.Context, cfg sdkprovider.Config, stage, function string, receipt any) (*sdkprovider.LogsResult, error)
}

// APIDispatchTransportPlugin adapts APIOps to sdkprovider.Plugin.
type APIDispatchTransportPlugin struct {
	id    string
	name  string
	ops   APIOps
	hooks APIDispatchHooks
}

// NewAPIOpsTransportPlugin creates a transport plugin backed by APIOps.
func NewAPIOpsTransportPlugin(providerID, providerName string, ops APIOps, hooks APIDispatchHooks) *APIDispatchTransportPlugin {
	return &APIDispatchTransportPlugin{id: providerID, name: providerName, ops: ops, hooks: hooks}
}

func (p *APIDispatchTransportPlugin) Meta() sdkprovider.Meta {
	return sdkprovider.Meta{
		Name:          p.id,
		PluginVersion: "1",
		Capabilities:  []string{"deploy", "remove", "invoke", "logs", "doctor"},
	}
}

func (p *APIDispatchTransportPlugin) ValidateConfig(ctx context.Context, req sdkprovider.ValidateConfigRequest) error {
	return nil
}

func (p *APIDispatchTransportPlugin) Doctor(ctx context.Context, req sdkprovider.DoctorRequest) (*sdkprovider.DoctorResult, error) {
	checks := []string{"api-dispatch provider configured"}
	if p.name != "" {
		checks = append(checks, "provider="+p.name)
	}
	if req.Stage != "" {
		checks = append(checks, "stage="+req.Stage)
	}
	return &sdkprovider.DoctorResult{Provider: p.id, Checks: checks}, nil
}

func (p *APIDispatchTransportPlugin) Plan(ctx context.Context, req sdkprovider.PlanRequest) (*sdkprovider.PlanResult, error) {
	return nil, fmt.Errorf("provider %q does not support plan", p.id)
}

func (p *APIDispatchTransportPlugin) Deploy(ctx context.Context, req sdkprovider.DeployRequest) (*sdkprovider.DeployResult, error) {
	return p.ops.Deploy(ctx, req.Config, req.Stage, req.Root)
}

func (p *APIDispatchTransportPlugin) Remove(ctx context.Context, req sdkprovider.RemoveRequest) (*sdkprovider.RemoveResult, error) {
	return p.ops.Remove(ctx, req.Config, req.Stage, req.Root, req.Receipt)
}

func (p *APIDispatchTransportPlugin) Invoke(ctx context.Context, req sdkprovider.InvokeRequest) (*sdkprovider.InvokeResult, error) {
	return p.ops.Invoke(ctx, req.Config, req.Stage, req.Function, req.Payload, nil)
}

func (p *APIDispatchTransportPlugin) Logs(ctx context.Context, req sdkprovider.LogsRequest) (*sdkprovider.LogsResult, error) {
	return p.ops.Logs(ctx, req.Config, req.Stage, req.Function, nil)
}

func (p *APIDispatchTransportPlugin) FetchMetrics(ctx context.Context, req sdkprovider.MetricsRequest) (*sdkprovider.MetricsResult, error) {
	if p.hooks.FetchMetrics == nil {
		return nil, fmt.Errorf("provider %q does not support metrics", p.id)
	}
	return p.hooks.FetchMetrics(ctx, req.Config, req.Stage)
}

func (p *APIDispatchTransportPlugin) FetchTraces(ctx context.Context, req sdkprovider.TracesRequest) (*sdkprovider.TracesResult, error) {
	if p.hooks.FetchTraces == nil {
		return nil, fmt.Errorf("provider %q does not support traces", p.id)
	}
	return p.hooks.FetchTraces(ctx, req.Config, req.Stage)
}

func (p *APIDispatchTransportPlugin) PrepareDevStream(ctx context.Context, req sdkprovider.DevStreamRequest) (*sdkprovider.DevStreamSession, error) {
	if p.hooks.PrepareDevStream == nil {
		return nil, fmt.Errorf("provider %q does not support dev stream", p.id)
	}
	return p.hooks.PrepareDevStream(ctx, req.Config, req.Stage, req.TunnelURL)
}

func (p *APIDispatchTransportPlugin) Recover(ctx context.Context, req sdkprovider.RecoveryRequest) (*sdkprovider.RecoveryResult, error) {
	if p.hooks.Recover == nil {
		return nil, fmt.Errorf("provider %q does not support recovery", p.id)
	}
	return p.hooks.Recover(ctx, req)
}

func (p *APIDispatchTransportPlugin) SyncOrchestrations(ctx context.Context, req sdkprovider.OrchestrationSyncRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	if p.hooks.SyncOrchestrations == nil {
		return nil, fmt.Errorf("provider %q does not support orchestration sync", p.id)
	}
	return p.hooks.SyncOrchestrations(ctx, req)
}

func (p *APIDispatchTransportPlugin) RemoveOrchestrations(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	if p.hooks.RemoveOrchestrations == nil {
		return nil, fmt.Errorf("provider %q does not support orchestration remove", p.id)
	}
	return p.hooks.RemoveOrchestrations(ctx, req)
}

func (p *APIDispatchTransportPlugin) InvokeOrchestration(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest) (*sdkprovider.InvokeResult, error) {
	if p.hooks.InvokeOrchestration == nil {
		return nil, fmt.Errorf("provider %q does not support orchestration invoke", p.id)
	}
	return p.hooks.InvokeOrchestration(ctx, req)
}

func (p *APIDispatchTransportPlugin) InspectOrchestrations(ctx context.Context, req sdkprovider.OrchestrationInspectRequest) (map[string]any, error) {
	if p.hooks.InspectOrchestrations == nil {
		return nil, fmt.Errorf("provider %q does not support orchestration inspect", p.id)
	}
	return p.hooks.InspectOrchestrations(ctx, req)
}
