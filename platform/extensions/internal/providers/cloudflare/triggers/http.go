package triggers

import (
	"context"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

// EnsureHTTP documents HTTP trigger for Cloudflare Workers. Routes are created in the deploy flow (Workers API); no separate trigger resource.
func EnsureHTTP(ctx context.Context, cfg *config.Config, stage, functionName string) error {
	return nil
}
