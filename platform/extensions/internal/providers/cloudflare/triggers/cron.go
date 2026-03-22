package triggers

import (
	"context"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

// EnsureCron documents cron trigger for Cloudflare Workers. Cron bindings are configured in the deploy flow (Workers API); no separate trigger resource here.
func EnsureCron(ctx context.Context, cfg *config.Config, stage, functionName, expression string) error {
	return nil
}
