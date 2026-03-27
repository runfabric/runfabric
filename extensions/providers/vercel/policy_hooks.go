package vercel

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func PrepareDevStreamPolicy(ctx context.Context, cfg sdkprovider.Config, stage, tunnelURL string) (*sdkprovider.DevStreamSession, error) {
	_ = ctx
	return sdkprovider.PrepareLifecycleDevStream("vercel", cfg, stage, tunnelURL)
}
