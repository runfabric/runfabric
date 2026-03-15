package deployrunner

import (
	"context"

	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/transactions"
	"github.com/runfabric/runfabric/providers"
)

type RunResult struct {
	DeployResult *providers.DeployResult
}

func Run(
	ctx context.Context,
	adapter providers.Adapter,
	cfg *config.Config,
	stage string,
	root string,
	journal *transactions.Journal,
) (*RunResult, error) {

	plan, err := adapter.BuildPlan(ctx, cfg, stage, root, journal)
	if err != nil {
		return nil, err
	}

	res, err := plan.Execute(ctx)
	if err != nil {
		_ = plan.Rollback(ctx)
		return nil, err
	}

	return &RunResult{
		DeployResult: res,
	}, nil
}
