package app

import (
	"context"
	"time"

	"github.com/runfabric/runfabric/engine/internal/controlplane"
	deployapi "github.com/runfabric/runfabric/engine/internal/deploy/api"
)

func Remove(configPath, stage, providerOverride string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}

	provider := ctx.Config.Provider.Name

	// Providers with API-based deploy: remove via provider API and clear receipt.
	if deployapi.HasRemover(provider) {
		return deployapi.Remove(context.Background(), provider, ctx.Config, ctx.Stage, ctx.RootDir)
	}

	// AWS and others: use control plane + registry (AWS has real Remove; others use lifecycle fallback).
	coord := &controlplane.Coordinator{
		Locks:     ctx.Backends.Locks,
		Journals:  ctx.Backends.Journals,
		Receipts:  ctx.Backends.Receipts,
		LeaseFor:  15 * time.Minute,
		Heartbeat: 30 * time.Second,
	}
	p, err := ctx.Registry.Get(provider)
	if err != nil {
		return nil, err
	}
	return controlplane.RunRemove(
		context.Background(),
		coord,
		p,
		ctx.Config,
		ctx.Stage,
		ctx.RootDir,
	)
}
