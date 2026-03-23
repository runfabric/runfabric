package vercel

import (
	"context"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
)

func PrepareDevStreamPolicy(ctx context.Context, cfg *providers.Config, stage, tunnelURL string) (*providers.DevStreamSession, error) {
	state, err := RedirectToTunnel(ctx, cfg, stage, tunnelURL)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	return providers.NewDevStreamSession(state.Mode, state.MissingPrereqs, state.StatusMessage, func(restoreCtx context.Context) error {
		return state.Restore(restoreCtx)
	}), nil
}
