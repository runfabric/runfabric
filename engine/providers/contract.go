package providers

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/backends"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/transactions"
)

type Adapter interface {
	Name() string

	BuildPlan(
		ctx context.Context,
		cfg *config.Config,
		stage string,
		root string,
		journal *transactions.Journal,
		receipts backends.ReceiptBackend,
	) (Plan, error)
}

type Plan interface {
	Execute(ctx context.Context) (*DeployResult, error)
	Rollback(ctx context.Context) error
}

type DeployResult struct {
	Service string `json:"service"`
	Stage   string `json:"stage"`
}
