package digitalocean

import (
	"context"

	devstreamtarget "github.com/runfabric/runfabric/platform/extensions/internal/providers/devstream"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// DevStreamState keeps a stable lifecycle-hook contract for DigitalOcean dev stream mode.
type DevStreamState = devstreamtarget.LifecycleState

// RedirectToTunnel prepares DigitalOcean dev-stream lifecycle state.
func RedirectToTunnel(ctx context.Context, cfg sdkprovider.Config, stage, tunnelURL string) (*DevStreamState, error) {
	_ = ctx
	return devstreamtarget.RedirectToTunnel("digitalocean-functions", cfg, stage, tunnelURL)
}
