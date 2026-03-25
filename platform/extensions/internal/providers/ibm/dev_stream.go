package ibm

import (
	"context"

	devstreamtarget "github.com/runfabric/runfabric/platform/extensions/internal/providers/devstream"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// DevStreamState keeps a stable lifecycle-hook contract for IBM OpenWhisk dev stream mode.
type DevStreamState = devstreamtarget.LifecycleState

// RedirectToTunnel prepares IBM OpenWhisk dev-stream lifecycle state.
func RedirectToTunnel(ctx context.Context, cfg sdkprovider.Config, stage, tunnelURL string) (*DevStreamState, error) {
	_ = ctx
	return devstreamtarget.RedirectToTunnel("ibm-openwhisk", cfg, stage, tunnelURL)
}
