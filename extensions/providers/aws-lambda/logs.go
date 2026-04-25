package aws

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *Provider) Logs(ctx context.Context, req sdkprovider.LogsRequest) (*sdkprovider.LogsResult, error) {
	_ = ctx

	return &sdkprovider.LogsResult{
		Provider: p.Name(),
		Function: req.Function,
		Lines:    []string{"logs are available in CloudWatch"},
	}, nil
}
