package resolution

import (
	"context"
	"fmt"
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	providercodec "github.com/runfabric/runfabric/platform/core/contracts/provider/codec"
	deployapi "github.com/runfabric/runfabric/platform/deploy/api"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
	planner "github.com/runfabric/runfabric/platform/planner/engine"
)

// APIDispatchProvider marks providers that should be executed through internal/deploy/api
// so operations that require project root/receipt context (invoke/logs/remove) can use
// deployapi directly.
type APIDispatchProvider interface {
	APIDispatchProvider()
}

// apiProviderAdapter connects API-dispatched providers into the provider registry so
// plan/doctor/lifecycle paths can resolve providers via a single boundary.
type apiProviderAdapter struct {
	name string
}

func (p *apiProviderAdapter) APIDispatchProvider() {}

func (p *apiProviderAdapter) Meta() providers.ProviderMeta {
	return providers.ProviderMeta{
		Name:            p.name,
		Capabilities:    []string{"deploy", "remove", "invoke", "logs", "doctor", "plan"},
		SupportsRuntime: []string{"nodejs", "python"},
	}
}

func (p *apiProviderAdapter) ValidateConfig(ctx context.Context, req providers.ValidateConfigRequest) error {
	return nil
}

func (p *apiProviderAdapter) Doctor(ctx context.Context, req providers.DoctorRequest) (*providers.DoctorResult, error) {
	return &providers.DoctorResult{
		Provider: p.name,
		Checks:   []string{"API provider; configure credentials per provider docs"},
	}, nil
}

func (p *apiProviderAdapter) Plan(ctx context.Context, req providers.PlanRequest) (*providers.PlanResult, error) {
	plan := &planner.Plan{
		Provider: p.name,
		Service:  req.Config.Service,
		Stage:    req.Stage,
		Actions:  []planner.PlanAction{{ID: "deploy", Type: planner.ActionCreate, Resource: planner.ResourceHTTPAPI, Name: "deploy", Description: "Deploy via provider API"}},
	}
	return &providers.PlanResult{Provider: p.name, Plan: plan}, nil
}

func (p *apiProviderAdapter) Deploy(ctx context.Context, req providers.DeployRequest) (*providers.DeployResult, error) {
	return deployapi.Run(ctx, p.name, req.Config, req.Stage, req.Root)
}

func (p *apiProviderAdapter) Remove(ctx context.Context, req providers.RemoveRequest) (*providers.RemoveResult, error) {
	return deployapi.Remove(ctx, p.name, req.Config, req.Stage, req.Root)
}

func (p *apiProviderAdapter) Invoke(ctx context.Context, req providers.InvokeRequest) (*providers.InvokeResult, error) {
	return deployapi.Invoke(ctx, p.name, req.Config, req.Stage, req.Function, req.Payload, "")
}

func (p *apiProviderAdapter) Logs(ctx context.Context, req providers.LogsRequest) (*providers.LogsResult, error) {
	return deployapi.Logs(ctx, p.name, req.Config, req.Stage, req.Function, "", nil)
}

func (p *apiProviderAdapter) FetchMetrics(ctx context.Context, req providers.MetricsRequest) (*providers.MetricsResult, error) {
	if h := providerpolicy.GetAPIHooks(p.name); h != nil && h.FetchMetrics != nil {
		tc, err := providercodec.FromCoreConfig(req.Config)
		if err != nil {
			return nil, err
		}
		r, err := h.FetchMetrics(ctx, tc, req.Stage)
		if err != nil {
			return nil, err
		}
		return &providers.MetricsResult{PerFunction: r.PerFunction, Message: r.Message}, nil
	}
	return &providers.MetricsResult{Message: "Metrics: use provider console for now; metrics export coming soon."}, nil
}

func (p *apiProviderAdapter) FetchTraces(ctx context.Context, req providers.TracesRequest) (*providers.TracesResult, error) {
	if h := providerpolicy.GetAPIHooks(p.name); h != nil && h.FetchTraces != nil {
		tc, err := providercodec.FromCoreConfig(req.Config)
		if err != nil {
			return nil, err
		}
		r, err := h.FetchTraces(ctx, tc, req.Stage)
		if err != nil {
			return nil, err
		}
		return &providers.TracesResult{Traces: r.Traces, Message: r.Message}, nil
	}
	return &providers.TracesResult{Message: "Traces: use provider console or runfabric logs for now; trace export coming soon."}, nil
}

func (p *apiProviderAdapter) PrepareDevStream(ctx context.Context, req providers.DevStreamRequest) (*providers.DevStreamSession, error) {
	if h := providerpolicy.GetAPIHooks(p.name); h != nil && h.PrepareDevStream != nil {
		tc, err := providercodec.FromCoreConfig(req.Config)
		if err != nil {
			return nil, err
		}
		r, err := h.PrepareDevStream(ctx, tc, req.Stage, req.TunnelURL)
		if err != nil || r == nil {
			return nil, err
		}
		return providers.NewDevStreamSession(r.EffectiveMode, r.MissingPrereqs, r.StatusMessage, r.Restore), nil
	}
	return nil, nil
}

func (p *apiProviderAdapter) Recover(ctx context.Context, req providers.RecoveryRequest) (*providers.RecoveryResult, error) {
	h := providerpolicy.GetAPIHooks(p.name)
	if h == nil || h.Recover == nil {
		return nil, fmt.Errorf("provider %q does not support recovery", p.name)
	}
	r, err := h.Recover(ctx, req)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (p *apiProviderAdapter) SyncOrchestrations(ctx context.Context, req providers.OrchestrationSyncRequest) (*providers.OrchestrationSyncResult, error) {
	if h := providerpolicy.GetAPIHooks(p.name); h != nil && h.SyncOrchestrations != nil {
		return h.SyncOrchestrations(ctx, req)
	}
	if p.name != "azure-functions" {
		return &providers.OrchestrationSyncResult{}, nil
	}
	return deployapi.SyncOrchestrations(ctx, p.name, req)
}

func (p *apiProviderAdapter) RemoveOrchestrations(ctx context.Context, req providers.OrchestrationRemoveRequest) (*providers.OrchestrationSyncResult, error) {
	if h := providerpolicy.GetAPIHooks(p.name); h != nil && h.RemoveOrchestrations != nil {
		return h.RemoveOrchestrations(ctx, req)
	}
	if p.name != "azure-functions" {
		return &providers.OrchestrationSyncResult{}, nil
	}
	return deployapi.RemoveOrchestrations(ctx, p.name, req)
}

func (p *apiProviderAdapter) InvokeOrchestration(ctx context.Context, req providers.OrchestrationInvokeRequest) (*providers.InvokeResult, error) {
	if h := providerpolicy.GetAPIHooks(p.name); h != nil && h.InvokeOrchestration != nil {
		return h.InvokeOrchestration(ctx, req)
	}
	if p.name != "azure-functions" {
		return nil, fmt.Errorf("provider %q does not support orchestration", p.name)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("orchestration name is required")
	}
	return deployapi.InvokeOrchestration(ctx, p.name, req.Config, req.Stage, req.Root, name, req.Payload)
}

func (p *apiProviderAdapter) InspectOrchestrations(ctx context.Context, req providers.OrchestrationInspectRequest) (map[string]any, error) {
	if h := providerpolicy.GetAPIHooks(p.name); h != nil && h.InspectOrchestrations != nil {
		return h.InspectOrchestrations(ctx, req)
	}
	if p.name != "azure-functions" {
		return map[string]any{}, nil
	}
	return deployapi.InspectOrchestrations(ctx, p.name, req.Config, req.Stage, req.Root)
}

// RegisterAPIProviders registers provider adapters for API-dispatched providers.
func RegisterAPIProviders(reg *providers.Registry) {
	for _, d := range providerpolicy.All() {
		if d.ExcludeFromAPIDispatch {
			continue
		}
		_ = reg.Register(&apiProviderAdapter{name: d.ID})
	}
}
