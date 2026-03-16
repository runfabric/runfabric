package digitalocean

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/backends"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/transactions"
	"github.com/runfabric/runfabric/engine/providers"
)

type Adapter struct{}

func NewAdapter() providers.Adapter {
	return &Adapter{}
}

func (a *Adapter) Name() string {
	return "digitalocean"
}

func (a *Adapter) BuildPlan(ctx context.Context, cfg *config.Config, stage string, root string, journal *transactions.Journal, _ backends.ReceiptBackend) (providers.Plan, error) {
	return &plan{}, nil
}

type plan struct{}

func (p *plan) Execute(ctx context.Context) (*providers.DeployResult, error) {
	return &providers.DeployResult{}, nil
}

func (p *plan) Rollback(ctx context.Context) error {
	return nil
}
