package fly

import (
	"context"

	"github.com/runfabric/runfabric/platform/extensions/internal/providers/fly/triggers"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func EnsureHTTP(ctx context.Context, cfg sdkprovider.Config, stage, functionName string) error {
	return triggers.EnsureHTTP(ctx, cfg, stage, functionName)
}
