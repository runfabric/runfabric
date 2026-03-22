package fly

import (
	"context"
	"fmt"

	coreconfig "github.com/runfabric/runfabric/platform/core/model/config"
	devstreamtarget "github.com/runfabric/runfabric/platform/extensions/internal/providers/devstream"
)

// DevStreamState keeps a stable lifecycle-hook contract for Fly Machines dev stream mode.
type DevStreamState = devstreamtarget.LifecycleState

// RedirectToTunnel prepares Fly Machines dev-stream lifecycle state.
func RedirectToTunnel(ctx context.Context, cfg *coreconfig.Config, stage, tunnelURL string) (*DevStreamState, error) {
	_ = ctx
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	return devstreamtarget.RedirectToTunnel("fly-machines", cfg, stage, tunnelURL)
}
