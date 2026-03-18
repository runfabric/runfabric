package netlify

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/provider/netlify/triggers"
)

func EnsureHTTP(ctx context.Context, cfg *config.Config, stage, functionName string) error {
	return triggers.EnsureHTTP(ctx, cfg, stage, functionName)
}

func EnsureCron(ctx context.Context, cfg *config.Config, stage, functionName, expression string) error {
	return triggers.EnsureCron(ctx, cfg, stage, functionName, expression)
}
