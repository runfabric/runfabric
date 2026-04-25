package alibaba

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// RedirectToTunnel prepares Alibaba FC dev-stream lifecycle state.
func RedirectToTunnel(ctx context.Context, cfg sdkprovider.Config, stage, tunnelURL string) (*sdkprovider.DevStreamSession, error) {
	return sdkprovider.PrepareLifecycleDevStream("alibaba-fc", cfg, stage, tunnelURL)
}
