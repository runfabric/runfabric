// Package triggers implements trigger hooks per Trigger Capability Matrix (http, cron for digitalocean-functions).
package triggers

import (
	"context"

	"github.com/runfabric/runfabric/internal/config"
)

// EnsureHTTP ensures HTTP trigger. No-op: HTTP is in the app spec at deploy time (functions component).
func EnsureHTTP(ctx context.Context, cfg *config.Config, stage, functionName string) error {
	return nil
}
