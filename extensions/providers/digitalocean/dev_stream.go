package digitalocean

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// RedirectToTunnel prepares DigitalOcean dev-stream lifecycle state.
func RedirectToTunnel(ctx context.Context, cfg sdkprovider.Config, stage, tunnelURL string) (*sdkprovider.DevStreamSession, error) {
	return sdkprovider.PrepareLifecycleDevStream("digitalocean-functions", cfg, stage, tunnelURL)
}
