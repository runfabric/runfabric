package deployrunner

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/deployexec"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/transactions"
)

type Adapter interface {
	BuildEngine(
		ctx context.Context,
		cfg *config.Config,
		stage string,
		root string,
		journal *transactions.Journal,
	) (*deployexec.Engine, *deployexec.Context, error)

	Finalize(
		ctx context.Context,
		cfg *config.Config,
		stage string,
		root string,
		execCtx *deployexec.Context,
	) (*providers.DeployResult, error)
}
