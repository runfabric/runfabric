package app

import (
	"fmt"

	statetypes "github.com/runfabric/runfabric/internal/state/types"
	corestate "github.com/runfabric/runfabric/platform/core/state/core"
)

// ReleasesResult is the result of Releases (deploy history per stage).
type ReleasesResult struct {
	Service  string                    `json:"service"`
	Releases []statetypes.ReleaseEntry `json:"releases"`
}

// Releases returns deployment history (stages and updated timestamps) from the receipt backend.
func Releases(configPath string) (any, error) {
	ctx, err := Bootstrap(configPath, "dev", "")
	if err != nil {
		return nil, err
	}
	list, err := ctx.Backends.Receipts.ListReleases()
	if err != nil {
		return nil, err
	}
	return &ReleasesResult{
		Service:  ctx.Config.Service,
		Releases: list,
	}, nil
}

// ReleaseHistoryResult is the result of ReleaseHistory: the retained past
// receipts for a single stage, newest first.
type ReleaseHistoryResult struct {
	Service string                   `json:"service"`
	Stage   string                   `json:"stage"`
	History []corestate.HistoryEntry `json:"history"`
}

// ReleaseHistory returns the retained past releases for a stage — each prior
// deployment's receipt is snapshotted before it is overwritten, so this is the
// engine's true per-stage deployment timeline (as opposed to Releases, which
// reports only the current head per stage).
func ReleaseHistory(configPath, stage string) (any, error) {
	if stage == "" {
		return nil, fmt.Errorf("stage is required")
	}
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	history, err := corestate.ListStageHistory(ctx.RootDir, stage)
	if err != nil {
		return nil, err
	}
	return &ReleaseHistoryResult{
		Service: ctx.Config.Service,
		Stage:   stage,
		History: history,
	}, nil
}
