package gcp

import (
	"context"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	planner "github.com/runfabric/runfabric/platform/core/planner/engine"
)

// Plan returns a deployment plan for GCP Cloud Functions using the shared planner.
func (p *Provider) Plan(ctx context.Context, req providers.PlanRequest) (*providers.PlanResult, error) {
	plan := planner.BuildPlan((*config.Config)(req.Config), req.Stage)
	plan.Provider = p.Name()
	return &providers.PlanResult{
		Provider: p.Name(),
		Plan:     plan,
		Warnings: nil,
	}, nil
}
