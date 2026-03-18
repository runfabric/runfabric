package deployrunner

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/backends"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/provider"
	"github.com/runfabric/runfabric/engine/internal/transactions"
)

type RunResult struct {
	DeployResult *provider.DeployResult
}

func Run(
	ctx context.Context,
	adapter provider.Adapter,
	cfg *config.Config,
	stage string,
	root string,
	journal *transactions.Journal,
	receipts backends.ReceiptBackend,
) (*RunResult, error) {

	plan, err := adapter.BuildPlan(ctx, cfg, stage, root, journal, receipts)
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
