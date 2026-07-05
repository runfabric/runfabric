package app

import (
	"context"

	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/core/state/runstore"
)

// resolveRunStore returns the run-state backend the user configured
// (RUNFABRIC_RUN_STORE env or extensions.runStore in runfabric.yml), falling
// back to the local filesystem backend rooted at root. cfg may be nil, in which
// case only the env override / local default apply.
//
// This is the read-side counterpart to newConfiguredRuntime: status/list
// readers must consult the same backend the runtime writes to, otherwise a
// dynamodb:// deployment would read empty local files.
func resolveRunStore(cfg *config.Config, root string) runstore.RunStore {
	configured := ""
	if cfg != nil {
		configured = config.ExtensionString(cfg, "runStore")
	}
	s, err := runstore.Resolve(configured, root)
	if err != nil {
		return runstore.NewLocalRunStore(root)
	}
	return s
}

// listRunsVia lists runs through the configured store (best effort; returns nil
// on error, matching the previous state.ListWorkflowRuns callers that ignored
// the error).
func listRunsVia(cfg *config.Config, root, stage string, limit int) []*state.WorkflowRun {
	runs, _ := resolveRunStore(cfg, root).List(context.Background(), stage, limit)
	return runs
}

// WorkflowRuns lists the stage's most recent workflow runs through the
// configured run store — the daemon-facing sibling of the per-run
// WorkflowStatus.
func WorkflowRuns(configPath, stage string, limit int) ([]*state.WorkflowRun, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	return listRunsVia(ctx.Config, ctx.RootDir, ctx.Stage, limit), nil
}

// loadRunVia loads a single run through the configured store.
func loadRunVia(cfg *config.Config, root, stage, runID string) (*state.WorkflowRun, error) {
	run, _, err := resolveRunStore(cfg, root).Load(context.Background(), stage, runID)
	return run, err
}
