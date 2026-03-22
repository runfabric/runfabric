package app

import (
	"context"

	awsprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/aws"
)

// PrepareDevStreamTunnel redirects the provider's invocation target (e.g. API Gateway) to tunnelURL
// for the given stage. Returns a restore function to call on exit to revert the change.
// If the provider does not support auto-wire (e.g. GCP, Cloudflare), returns (nil, nil) and the
// local server still runs; see DEV_LIVE_STREAM.md for provider-specific behavior.
func PrepareDevStreamTunnel(configPath, stage, tunnelURL string) (restore func(), err error) {
	if tunnelURL == "" {
		return nil, nil
	}
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	p := ctx.Config.Provider.Name
	// Only AWS has auto-wire today; GCP/Cloudflare/others run local server only (no restore).
	if p != "aws-lambda" {
		return nil, nil
	}
	state, err := awsprovider.RedirectToTunnel(context.Background(), ctx.Config, stage, tunnelURL)
	if err != nil {
		return nil, err
	}
	region := ctx.Config.Provider.Region
	return func() {
		_ = state.Restore(context.Background(), region)
	}, nil
}
