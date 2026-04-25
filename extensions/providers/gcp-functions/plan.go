package gcp

import (
	"context"
	"fmt"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

type Planner struct{}

func (Planner) Plan(ctx context.Context, cfg sdkprovider.Config, stage, root string) (*sdkprovider.PlanResult, error) {
	_ = ctx
	_ = root
	service := sdkprovider.Service(cfg)
	if service == "" {
		service = "service"
	}
	steps := []string{}
	for name := range sdkprovider.Functions(cfg) {
		steps = append(steps, fmt.Sprintf("deploy function %s-%s-%s", service, stage, name))
	}
	if len(steps) == 0 {
		steps = append(steps, "no functions configured")
	}
	return &sdkprovider.PlanResult{
		Provider: ProviderID,
		Plan: map[string]any{
			"provider": ProviderID,
			"stage":    stage,
			"steps":    steps,
		},
	}, nil
}
