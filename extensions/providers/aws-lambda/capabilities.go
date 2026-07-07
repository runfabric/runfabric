package aws

import (
	"context"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// FetchMetrics and FetchTraces live in observability.go; Recover in recover_ops.go.

func (p *Provider) PrepareDevStream(ctx context.Context, req sdkprovider.DevStreamRequest) (*sdkprovider.DevStreamSession, error) {
	_ = strings.TrimSpace(req.Region)
	return sdkprovider.PrepareLifecycleDevStream(ProviderID, req.Config, req.Stage, req.TunnelURL)
}
