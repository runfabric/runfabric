package aws

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *Provider) Doctor(ctx context.Context, req sdkprovider.DoctorRequest) (*sdkprovider.DoctorResult, error) {
	_ = ctx
	checks := []string{
		"Provider: aws-lambda",
		"Region: " + sdkprovider.ProviderRegion(req.Config),
		"Runtime: " + sdkprovider.ProviderRuntime(req.Config),
	}
	if sdkprovider.Env("AWS_ACCESS_KEY_ID") == "" || sdkprovider.Env("AWS_SECRET_ACCESS_KEY") == "" {
		checks = append(checks, "AWS credentials are not set in environment")
	} else {
		checks = append(checks, "AWS credentials are present")
	}

	return &sdkprovider.DoctorResult{
		Provider: p.Name(),
		Checks:   checks,
	}, nil
}
