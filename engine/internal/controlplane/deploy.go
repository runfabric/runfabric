package controlplane

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/deployrunner"
	"github.com/runfabric/runfabric/engine/internal/extensions/provider"
)

func RunDeploy(
	ctx context.Context,
	coord *Coordinator,
	adapter provider.Adapter,
	cfg *config.Config,
	stage string,
	root string,
) (*provider.DeployResult, error) {
	EmitEvent("deploy-start", cfg.Service, stage, "deploy started", nil)
	run, err := coord.AcquireRunContext(ctx, cfg.Service, stage, "deploy")
	if err != nil {
		return nil, err
	}
	defer func() { _ = coord.Close(run) }()

	if err := run.Journal.IncrementAttempt(); err != nil {
		EmitEvent("deploy-failed", cfg.Service, stage, err.Error(), nil)
		return nil, err
	}

	result, err := deployrunner.Run(ctx, adapter, cfg, stage, root, run.Journal, coord.Receipts)
	if err != nil {
		_ = run.Journal.MarkRollingBack()
		EmitEvent("deploy-failed", cfg.Service, stage, err.Error(), nil)
		return nil, err
	}

	if err := AbortIfLeaseLost(ctx, run.Lock, cfg.Service, stage); err != nil {
		_ = run.Journal.Checkpoint("lease", "lost")
		EmitEvent("lease-lost", cfg.Service, stage, "heartbeat renewal failed", nil)
		return nil, err
	}

	if err := run.Journal.MarkCompleted(); err != nil {
		return nil, fmt.Errorf("mark journal completed: %w", err)
	}
	_ = run.Journal.Delete()
	EmitEvent("deploy-complete", cfg.Service, stage, "deploy completed", nil)
	return result.DeployResult, nil
}
