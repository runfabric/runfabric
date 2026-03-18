// Package app provides apiProviderStub so API-based providers (vercel, netlify, etc.)
// are registered in the extensions registry and support plan/doctor; deploy/remove/invoke/logs
// are delegated to deployapi.
package app

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/config"
	deployapi "github.com/runfabric/runfabric/engine/internal/deploy/api"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/planner"
)

// apiProviderStub implements providers.Provider for API-based deploy targets (vercel, netlify, etc.).
// Doctor and Plan return minimal results; Deploy, Remove, Invoke, Logs delegate to deployapi.
type apiProviderStub struct {
	name string
}

func (p *apiProviderStub) Name() string {
	return p.name
}

func (p *apiProviderStub) Doctor(cfg *config.Config, stage string) (*providers.DoctorResult, error) {
	return &providers.DoctorResult{
		Provider: p.name,
		Checks:   []string{"API provider; configure credentials per provider docs"},
	}, nil
}

func (p *apiProviderStub) Plan(cfg *config.Config, stage, root string) (*providers.PlanResult, error) {
	plan := &planner.Plan{
		Provider: p.name,
		Service:  cfg.Service,
		Stage:    stage,
		Actions:  []planner.PlanAction{{ID: "deploy", Type: planner.ActionCreate, Resource: planner.ResourceHTTPAPI, Name: "deploy", Description: "Deploy via provider API"}},
	}
	return &providers.PlanResult{Provider: p.name, Plan: plan}, nil
}

func (p *apiProviderStub) Deploy(cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	return deployapi.Run(context.Background(), p.name, cfg, stage, root)
}

func (p *apiProviderStub) Remove(cfg *config.Config, stage, root string) (*providers.RemoveResult, error) {
	return deployapi.Remove(context.Background(), p.name, cfg, stage, root)
}

func (p *apiProviderStub) Invoke(cfg *config.Config, stage, function string, payload []byte) (*providers.InvokeResult, error) {
	return deployapi.Invoke(context.Background(), p.name, cfg, stage, function, payload, "")
}

func (p *apiProviderStub) Logs(cfg *config.Config, stage, function string) (*providers.LogsResult, error) {
	// Root is not available on the Provider interface; pass "" so deployapi may load receipt when root is known internally.
	return deployapi.Logs(context.Background(), p.name, cfg, stage, function, "", nil)
}

// RegisterAPIProviders registers stub providers for all API-based deploy targets so reg.Get(name) works for plan/doctor and lifecycle paths.
// Used by Bootstrap and by CLI plugin list so the registry is consistent.
func RegisterAPIProviders(reg *providers.Registry) {
	for _, name := range deployapi.APIProviderNames() {
		// Skip providers already registered by bootstrap (aws, aws-lambda, gcp-functions).
		if name == "aws" || name == "aws-lambda" || name == "gcp-functions" {
			continue
		}
		reg.Register(&apiProviderStub{name: name})
	}
}
