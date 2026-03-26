package aws

import (
	"context"
	"fmt"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *Provider) Deploy(ctx context.Context, req sdkprovider.DeployRequest) (*sdkprovider.DeployResult, error) {
	_ = ctx
	_ = req
	return nil, fmt.Errorf("direct provider deploy is no longer supported; use the control-plane deploy path")
}
