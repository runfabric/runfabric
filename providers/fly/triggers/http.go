package triggers

import (
	"context"

	"github.com/runfabric/runfabric/internal/config"
)

// EnsureHTTP documents HTTP trigger for Fly. The app URL is the HTTP endpoint; no separate trigger resource.
func EnsureHTTP(ctx context.Context, cfg *config.Config, stage, functionName string) error {
	return nil
}
