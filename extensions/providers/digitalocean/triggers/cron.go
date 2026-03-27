package triggers

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// EnsureCron ensures cron trigger. No-op: cron is in the app spec at deploy time (jobs with kind SCHEDULED).
func EnsureCron(ctx context.Context, cfg sdkprovider.Config, stage, functionName, expression string) error {
	return nil
}
