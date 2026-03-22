package ibm

import (
	"context"
	"fmt"

	coreconfig "github.com/runfabric/runfabric/platform/core/model/config"
	devstreamtarget "github.com/runfabric/runfabric/platform/extensions/internal/providers/devstream"
)

// DevStreamState keeps a stable lifecycle-hook contract for IBM OpenWhisk dev stream mode.
type DevStreamState = devstreamtarget.LifecycleState

// RedirectToTunnel prepares IBM OpenWhisk dev-stream lifecycle state.
func RedirectToTunnel(ctx context.Context, cfg *coreconfig.Config, stage, tunnelURL string) (*DevStreamState, error) {
	_ = ctx
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	return devstreamtarget.RedirectToTunnel("ibm-openwhisk", cfg, stage, tunnelURL)
}
