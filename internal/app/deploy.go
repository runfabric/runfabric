package app

import (
	"context"
	"time"

	"github.com/runfabric/runfabric/internal/controlplane"
	deployapi "github.com/runfabric/runfabric/internal/deploy/api"
	"github.com/runfabric/runfabric/internal/lifecycle"
	awsprovider "github.com/runfabric/runfabric/providers/aws"
)

func Deploy(configPath, stage string) (any, error) {
	ctx, err := Bootstrap(configPath, stage)
	if err != nil {
		return nil, err
	}

	provider := ctx.Config.Provider.Name

	// AWS: full controlplane + adapter (real deploy via SDK).
	if provider == "aws" || provider == "aws-lambda" {
		coord := &controlplane.Coordinator{
			Locks:     ctx.Backends.Locks,
			Journals:  ctx.Backends.Journals,
			Receipts:  ctx.Backends.Receipts,
			LeaseFor:  15 * time.Minute,
			Heartbeat: 30 * time.Second,
		}
		return controlplane.RunDeploy(
			context.Background(),
			coord,
			awsprovider.NewAdapter(),
			ctx.Config,
			ctx.Stage,
			ctx.RootDir,
		)
	}

	// Other providers: real deploy via REST/SDK (no CLI). No wrangler, vercel, fly, gcloud, kubectl, etc. required.
	if deployapi.HasRunner(provider) {
		result, err := deployapi.Run(context.Background(), provider, ctx.Config, ctx.Stage, ctx.RootDir)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	// Fallback: lifecycle (simulated deploy for providers without API implementation).
	result, err := lifecycle.Deploy(ctx.Registry, ctx.Config, ctx.Stage, ctx.RootDir)
	if err != nil {
		return nil, err
	}
	return result, nil
}
