package triggers

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// EnsureCron documents cron trigger for Cloudflare Workers. Cron bindings are configured in the deploy flow (Workers API); no separate trigger resource here.
func EnsureCron(ctx context.Context, cfg sdkprovider.Config, stage, functionName, expression string) error {
	return nil
}
