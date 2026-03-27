package triggers

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// EnsureHTTP documents HTTP trigger for Cloudflare Workers. Routes are created in the deploy flow (Workers API); no separate trigger resource.
func EnsureHTTP(ctx context.Context, cfg sdkprovider.Config, stage, functionName string) error {
	return nil
}
