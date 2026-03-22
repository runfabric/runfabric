package triggers

import (
	"context"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

func EnsureCron(ctx context.Context, cfg *config.Config, stage, functionName, expression string) error {
	return nil
}
