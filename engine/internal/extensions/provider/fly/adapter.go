package fly

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/backends"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/provider"
	"github.com/runfabric/runfabric/engine/internal/transactions"
)

type Adapter struct{}

func NewAdapter() provider.Adapter {
	return &Adapter{}
}

func (a *Adapter) Name() string {
	return "fly"
}

func (a *Adapter) BuildPlan(ctx context.Context, cfg *config.Config, stage string, root string, journal *transactions.Journal, _ backends.ReceiptBackend) (provider.Plan, error) {
	return &plan{}, nil
}

type plan struct{}

func (p *plan) Execute(ctx context.Context) (*provider.DeployResult, error) {
	return &provider.DeployResult{}, nil
}

func (p *plan) Rollback(ctx context.Context) error {
	return nil
}
