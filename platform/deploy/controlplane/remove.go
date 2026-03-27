package controlplane

import (
	"context"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

type RemoveExecutor interface {
	Remove(cfg *config.Config, stage, root string) (*providers.RemoveResult, error)
}

func RunRemove(
	ctx context.Context,
	coord *Coordinator,
	executor RemoveExecutor,
	cfg *config.Config,
	stage string,
	root string,
) (*providers.RemoveResult, error) {
	run, err := coord.AcquireRunContext(ctx, cfg.Service, stage, "remove")
	if err != nil {
		return nil, err
	}
	defer func() { _ = coord.Close(run) }()

	result, err := executor.Remove(cfg, stage, root)
	if err != nil {
		_ = run.Journal.MarkRollingBack()
		return nil, err
	}

	if err := FailIfLeaseLost(ctx, run.Lock, cfg.Service, stage); err != nil {
		_ = run.Journal.Checkpoint("lease", "lost")
		return nil, err
	}

	if err := run.Journal.MarkCompleted(); err != nil {
		return nil, err
	}
	_ = run.Journal.Delete()

	return result, nil
}
