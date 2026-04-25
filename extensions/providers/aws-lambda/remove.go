package aws

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *Provider) Remove(ctx context.Context, req sdkprovider.RemoveRequest) (*sdkprovider.RemoveResult, error) {
	_ = ctx
	_ = req

	return &sdkprovider.RemoveResult{
		Provider: p.Name(),
		Removed:  true,
	}, nil
}
