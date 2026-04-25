package exec

import (
	"context"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/core/state/transactions"
)

// RunDeploy executes a provider deploy wrapped in journal-tracked phases.
// journal may be nil — phases still run but no checkpoints are persisted.
// On success the journal is marked completed. On failure it stays active for retry.
func RunDeploy(
	ctx context.Context,
	cfg *config.Config,
	stage, root string,
	faults FaultConfig,
	journal *transactions.Journal,
	deployFn func(ctx context.Context) (*providers.DeployResult, error),
) (*providers.DeployResult, error) {
	execCtx := &Context{
		Root:   root,
		Config: cfg,
		Stage:  stage,
		Faults: faults,
	}

	engine := &Engine{
		Journal: journal,
		Phases: []Phase{
			PhaseFunc{
				PhaseName: CheckpointDeployFunctions,
				Fn: func(ctx context.Context, ec *Context) error {
					result, err := deployFn(ctx)
					if err != nil {
						return err
					}
					ec.Result = result
					return nil
				},
			},
		},
	}

	if journal != nil {
		_ = journal.IncrementAttempt()
	}

	if err := engine.Run(ctx, execCtx); err != nil {
		return nil, err
	}

	if journal != nil {
		_ = journal.MarkCompleted()
	}

	return execCtx.Result, nil
}

// OpenDeployJournal returns an existing active journal for resume, or a new one.
// Returns nil (not an error) when root is empty, so callers don't need to gate on it.
func OpenDeployJournal(service, stage, root string) *transactions.Journal {
	if root == "" {
		return nil
	}
	backend := transactions.NewFileBackend(root)
	if file, err := backend.Load(service, stage); err == nil && file != nil && file.Status == transactions.StatusActive {
		return transactions.NewJournalFromFile(file, backend)
	}
	return transactions.NewJournal(service, stage, "deploy", backend)
}
