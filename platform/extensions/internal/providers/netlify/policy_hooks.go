package netlify

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func PrepareDevStreamPolicy(ctx context.Context, cfg sdkprovider.Config, stage, tunnelURL string) (*sdkprovider.DevStreamSession, error) {
	state, err := RedirectToTunnel(ctx, cfg, stage, tunnelURL)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	return &sdkprovider.DevStreamSession{EffectiveMode: state.Mode, MissingPrereqs: append([]string(nil), state.MissingPrereqs...), StatusMessage: state.StatusMessage}, nil
}
