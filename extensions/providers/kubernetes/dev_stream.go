package kubernetes

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// RedirectToTunnel prepares Kubernetes dev-stream lifecycle state.
func RedirectToTunnel(ctx context.Context, cfg sdkprovider.Config, stage, tunnelURL string) (*sdkprovider.DevStreamSession, error) {
	return sdkprovider.PrepareLifecycleDevStream("kubernetes", cfg, stage, tunnelURL)
}
