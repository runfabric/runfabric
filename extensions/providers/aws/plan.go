package aws

import (
	"context"
	"fmt"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *Provider) Plan(ctx context.Context, req sdkprovider.PlanRequest) (*sdkprovider.PlanResult, error) {
	_ = ctx
	service := sdkprovider.Service(req.Config)
	if service == "" {
		service = "service"
	}
	steps := []string{}
	for fn := range sdkprovider.Functions(req.Config) {
		steps = append(steps, fmt.Sprintf("deploy function %s-%s-%s", service, req.Stage, fn))
	}
	if len(steps) == 0 {
		steps = append(steps, "no functions configured")
	}
	return &sdkprovider.PlanResult{
		Provider: p.Name(),
		Plan: map[string]any{
			"provider": p.Name(),
			"stage":    req.Stage,
			"steps":    steps,
		},
	}, nil
}
