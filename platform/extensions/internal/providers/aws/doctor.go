package aws

import (
	"context"
	"fmt"
	"time"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
)

func (p *Provider) Doctor(ctx context.Context, req providers.DoctorRequest) (*providers.DoctorResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cfg := req.Config

	clients, err := loadClients(ctx, cfg.Provider.Region)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	checks := []string{
		"AWS config loaded",
		"Account ID resolved: " + clients.AccountID,
		"Region configured: " + cfg.Provider.Region,
		"Runtime configured: " + cfg.Provider.Runtime,
	}

	stepFunctions, err := stepFunctionsFromConfig(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("step functions config validation failed: %w", err)
	}
	if len(stepFunctions) > 0 {
		checks = append(checks, fmt.Sprintf("Step Functions config entries: %d", len(stepFunctions)))
	}

	_, err = clients.IAM.ListRoles(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("iam access check failed: %w", err)
	}
	checks = append(checks, "IAM access check passed")

	_, err = clients.Lambda.ListFunctions(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("lambda access check failed: %w", err)
	}
	checks = append(checks, "Lambda access check passed")

	_, err = clients.Logs.DescribeLogGroups(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("cloudwatch logs access check failed: %w", err)
	}
	checks = append(checks, "CloudWatch Logs access check passed")

	_, err = clients.APIGW.GetApis(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("api gateway access check failed: %w", err)
	}
	checks = append(checks, "API Gateway access check passed")

	if len(stepFunctions) > 0 {
		_, err = clients.SFN.ListStateMachines(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("step functions access check failed: %w", err)
		}
		checks = append(checks, "Step Functions access check passed")
	}

	return &providers.DoctorResult{
		Provider: p.Name(),
		Checks:   checks,
	}, nil
}
