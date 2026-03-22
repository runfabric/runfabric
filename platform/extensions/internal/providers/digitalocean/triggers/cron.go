package triggers

import (
	"context"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

// EnsureCron ensures cron trigger. No-op: cron is in the app spec at deploy time (jobs with kind SCHEDULED).
func EnsureCron(ctx context.Context, cfg *config.Config, stage, functionName, expression string) error {
	return nil
}
