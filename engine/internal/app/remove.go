package app

import (
	"context"
	"time"

	"github.com/runfabric/runfabric/engine/internal/controlplane"
	deployapi "github.com/runfabric/runfabric/engine/internal/deploy/api"
	"github.com/runfabric/runfabric/engine/internal/lifecycle"
)

func Remove(configPath, stage, providerOverride string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}

	provider, err := resolveProvider(ctx)
	if err != nil {
		return nil, err
	}

	if provider.mode == dispatchInternal {
		coord := &controlplane.Coordinator{
			Locks:     ctx.Backends.Locks,
			Journals:  ctx.Backends.Journals,
			Receipts:  ctx.Backends.Receipts,
			LeaseFor:  15 * time.Minute,
			Heartbeat: 30 * time.Second,
		}
		return controlplane.RunRemove(
			context.Background(),
			coord,
			provider.provider,
			ctx.Config,
			ctx.Stage,
			ctx.RootDir,
		)
	}

	if provider.mode == dispatchAPI {
		return deployapi.Remove(context.Background(), provider.name, ctx.Config, ctx.Stage, ctx.RootDir)
	}

	return lifecycle.Remove(ctx.Registry, ctx.Config, ctx.Stage, ctx.RootDir)
}
