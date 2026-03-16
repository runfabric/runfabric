package aws

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/backends"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/transactions"
	"github.com/runfabric/runfabric/engine/providers"
)

type Adapter struct{}

func NewAdapter() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Name() string {
	return "aws"
}

func (a *Adapter) BuildPlan(
	ctx context.Context,
	cfg *config.Config,
	stage string,
	root string,
	journal *transactions.Journal,
	receipts backends.ReceiptBackend,
) (providers.Plan, error) {

	return NewDeployPlan(cfg, stage, root, journal, receipts), nil
}
