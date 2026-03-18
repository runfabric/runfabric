package triggers

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/config"
)

func EnsureCron(ctx context.Context, cfg *config.Config, stage, functionName, expression string) error {
	return nil
}
