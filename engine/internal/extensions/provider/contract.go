// Package provider holds the Adapter contract and built-in provider implementations (aws, gcp, vercel, etc.).
package provider

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/backends"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/transactions"
)

// Adapter builds a deploy plan for a provider (e.g. AWS, Vercel).
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

// Plan is the result of BuildPlan; Execute runs deploy, Rollback undoes it.
type Plan interface {
	Execute(ctx context.Context) (*DeployResult, error)
	Rollback(ctx context.Context) error
}

// DeployResult is the minimal result from Plan.Execute (used by deployrunner).
type DeployResult struct {
	Service string `json:"service"`
	Stage   string `json:"stage"`
}
