package aws

import (
	"context"

	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/transactions"
	"github.com/runfabric/runfabric/providers"
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
) (providers.Plan, error) {

	return NewDeployPlan(cfg, stage, root, journal), nil
}
