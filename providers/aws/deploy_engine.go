package aws

import (
	"context"

	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/transactions"
)

type DeployEngine struct{}

func NewDeployEngine() *DeployEngine {
	return &DeployEngine{}
}

func (e *DeployEngine) DeployFunctions(
	ctx context.Context,
	cfg *config.Config,
	stage string,
	root string,
	journal *transactions.Journal,
) error {

	for name, fn := range cfg.Functions {

		// checkpoint for resumability
		if err := journal.Checkpoint("function", name); err != nil {
			return err
		}

		if err := e.deploySingleFunction(ctx, cfg, stage, root, name, fn); err != nil {
			return err
		}
	}

	return nil
}

func (e *DeployEngine) deploySingleFunction(
	ctx context.Context,
	cfg *config.Config,
	stage string,
	root string,
	name string,
	fn config.FunctionConfig,
) error {
	// AWS deploy uses the phase engine in deploy_plan.Execute (newResumeDependencies + newDeployEngine).
	// This DeployEngine is only used by legacy or alternate code paths; the main path does not call it.
	_ = ctx
	_ = cfg
	_ = stage
	_ = root
	_ = name
	_ = fn
	return nil
}

func (e *DeployEngine) Rollback(
	ctx context.Context,
	cfg *config.Config,
	stage string,
	root string,
	journal *transactions.Journal,
) error {
	_ = ctx
	_ = cfg
	_ = stage
	_ = root
	_ = journal
	return nil
}
