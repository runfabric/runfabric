package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/providers"
)

func (p *Provider) Doctor(cfg *config.Config, stage string) (*providers.DoctorResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

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

	return &providers.DoctorResult{
		Provider: p.Name(),
		Checks:   checks,
	}, nil
}
