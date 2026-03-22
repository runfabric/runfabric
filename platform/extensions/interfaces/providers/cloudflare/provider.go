package cloudflare

import (
	"context"
	"fmt"

	coreconfig "github.com/runfabric/runfabric/platform/core/model/config"
	cloudflaretarget "github.com/runfabric/runfabric/platform/extensions/internal/providers/cloudflare"
)

// DevStreamState holds state for redirecting Cloudflare Workers to a tunnel and restoring on exit.
type DevStreamState = cloudflaretarget.DevStreamState

// RedirectToTunnel finds the Cloudflare Worker for the service/stage and sets up tunnel redirection.
func RedirectToTunnel(ctx context.Context, cfg *coreconfig.Config, stage, tunnelURL string) (*DevStreamState, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	return cloudflaretarget.RedirectToTunnel(ctx, cfg, stage, tunnelURL)
}
