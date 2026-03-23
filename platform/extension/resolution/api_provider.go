package resolution

import (
	"context"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	planner "github.com/runfabric/runfabric/platform/core/planner/engine"
	deployapi "github.com/runfabric/runfabric/platform/deploy/core/api"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

// APIDispatchProvider marks providers that should be executed through internal/deploy/api
// so operations that require project root/receipt context (invoke/logs/remove) can use
// deployapi directly.
type APIDispatchProvider interface {
	APIDispatchProvider()
}

// apiProviderAdapter bridges API-dispatched providers into the provider registry so
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

// RegisterAPIProviders registers provider adapters for API-dispatched providers.
func RegisterAPIProviders(reg *providers.Registry) {
	for _, d := range providerpolicy.All() {
		if d.ExcludeFromAPIDispatch {
			continue
		}
		_ = reg.Register(&apiProviderAdapter{name: d.ID})
	}
}
