package resolution

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/config"
	deployapi "github.com/runfabric/runfabric/engine/internal/deploy/api"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/planner"
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

func (p *apiProviderAdapter) Name() string {
	return p.name
}

func (p *apiProviderAdapter) Doctor(cfg *config.Config, stage string) (*providers.DoctorResult, error) {
	return &providers.DoctorResult{
		Provider: p.name,
		Checks:   []string{"API provider; configure credentials per provider docs"},
	}, nil
}

func (p *apiProviderAdapter) Plan(cfg *config.Config, stage, root string) (*providers.PlanResult, error) {
	plan := &planner.Plan{
		Provider: p.name,
		Service:  cfg.Service,
		Stage:    stage,
		Actions:  []planner.PlanAction{{ID: "deploy", Type: planner.ActionCreate, Resource: planner.ResourceHTTPAPI, Name: "deploy", Description: "Deploy via provider API"}},
	}
	return &providers.PlanResult{Provider: p.name, Plan: plan}, nil
}

func (p *apiProviderAdapter) Deploy(cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	return deployapi.Run(context.Background(), p.name, cfg, stage, root)
}

func (p *apiProviderAdapter) Remove(cfg *config.Config, stage, root string) (*providers.RemoveResult, error) {
	return deployapi.Remove(context.Background(), p.name, cfg, stage, root)
}

func (p *apiProviderAdapter) Invoke(cfg *config.Config, stage, function string, payload []byte) (*providers.InvokeResult, error) {
	return deployapi.Invoke(context.Background(), p.name, cfg, stage, function, payload, "")
}

func (p *apiProviderAdapter) Logs(cfg *config.Config, stage, function string) (*providers.LogsResult, error) {
	return deployapi.Logs(context.Background(), p.name, cfg, stage, function, "", nil)
}

// RegisterAPIProviders registers provider adapters for API-dispatched providers.
func RegisterAPIProviders(reg *providers.Registry) {
	for _, name := range deployapi.APIProviderNames() {
		// Keep built-in/internal providers authoritative.
		if name == "aws" || name == "aws-lambda" || name == "gcp-functions" {
			continue
		}
		reg.Register(&apiProviderAdapter{name: name})
	}
}
