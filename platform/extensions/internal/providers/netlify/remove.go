package netlify

import (
	"context"
	"fmt"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

// Remover deletes the site via Netlify API (DELETE /sites/{id}).
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	siteID := receipt.Outputs["site_id"]
	if siteID == "" {
		siteID = receipt.Metadata["site_id"]
	}
	if siteID == "" {
		return nil, fmt.Errorf("receipt missing site_id; cannot remove Netlify site")
	}
	url := netlifyAPI + "/sites/" + siteID
	if err := apiutil.DoDelete(ctx, url, "NETLIFY_AUTH_TOKEN"); err != nil {
		return nil, fmt.Errorf("netlify delete site: %w", err)
	}
	return &providers.RemoveResult{Provider: "netlify", Removed: true}, nil
}
