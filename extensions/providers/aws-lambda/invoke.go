package aws

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *Provider) Invoke(ctx context.Context, req sdkprovider.InvokeRequest) (*sdkprovider.InvokeResult, error) {
	_ = ctx

	return &sdkprovider.InvokeResult{
		Provider: p.Name(),
		Function: req.Function,
		Output:   string(req.Payload),
	}, nil
}
