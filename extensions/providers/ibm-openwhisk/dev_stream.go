package ibm

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// RedirectToTunnel prepares IBM OpenWhisk dev-stream lifecycle state.
func RedirectToTunnel(ctx context.Context, cfg sdkprovider.Config, stage, tunnelURL string) (*sdkprovider.DevStreamSession, error) {
	return sdkprovider.PrepareLifecycleDevStream("ibm-openwhisk", cfg, stage, tunnelURL)
}
