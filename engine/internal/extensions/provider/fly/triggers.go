package fly

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/provider/fly/triggers"
)

func EnsureHTTP(ctx context.Context, cfg *config.Config, stage, functionName string) error {
	return triggers.EnsureHTTP(ctx, cfg, stage, functionName)
}
