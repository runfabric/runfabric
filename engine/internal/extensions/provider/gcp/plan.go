package gcp

import (
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/planner"
)

// Plan returns a deployment plan for GCP Cloud Functions using the shared planner.
func (p *Provider) Plan(cfg *providers.Config, stage, root string) (*providers.PlanResult, error) {
	plan := planner.BuildPlan((*config.Config)(cfg), stage)
	plan.Provider = p.Name()
	return &providers.PlanResult{
		Provider: p.Name(),
		Plan:     plan,
		Warnings: nil,
	}, nil
}
