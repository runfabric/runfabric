package digitalocean

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func PrepareDevStreamPolicy(ctx context.Context, cfg sdkprovider.Config, stage, tunnelURL string) (*sdkprovider.DevStreamSession, error) {
	return sdkprovider.PrepareLifecycleDevStream("digitalocean-functions", cfg, stage, tunnelURL)
}
