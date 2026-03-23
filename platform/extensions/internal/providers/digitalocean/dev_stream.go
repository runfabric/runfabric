package digitalocean

import (
	"context"

	"github.com/runfabric/runfabric/platform/core/model/config"
	devstreamtarget "github.com/runfabric/runfabric/platform/extensions/internal/providers/devstream"
)

// DevStreamState keeps a stable lifecycle-hook contract for DigitalOcean dev stream mode.
type DevStreamState = devstreamtarget.LifecycleState

// RedirectToTunnel prepares DigitalOcean dev-stream lifecycle state.
func RedirectToTunnel(ctx context.Context, cfg *config.Config, stage, tunnelURL string) (*DevStreamState, error) {
	_ = ctx
	return devstreamtarget.RedirectToTunnel("digitalocean-functions", cfg, stage, tunnelURL)
}
