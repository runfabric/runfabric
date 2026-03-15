package cloudflare

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/internal/apiutil"
	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
	"github.com/runfabric/runfabric/internal/state"
)

// Remover deletes the Worker via Cloudflare API.
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	if apiutil.Env("CLOUDFLARE_ACCOUNT_ID") == "" || apiutil.Env("CLOUDFLARE_API_TOKEN") == "" {
		return nil, fmt.Errorf("CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_API_TOKEN are required")
	}
	accountID := apiutil.Env("CLOUDFLARE_ACCOUNT_ID")
	name := receipt.Metadata["worker"]
	if name == "" {
		name = fmt.Sprintf("%s-%s", cfg.Service, stage)
	}
	url := cfAPI + "/accounts/" + accountID + "/workers/scripts/" + name
	if err := apiutil.DoDelete(ctx, url, "CLOUDFLARE_API_TOKEN"); err != nil {
		return nil, fmt.Errorf("cloudflare delete worker: %w", err)
	}
	return &providers.RemoveResult{Provider: "cloudflare-workers", Removed: true}, nil
}
