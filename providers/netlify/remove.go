package netlify

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/internal/apiutil"
	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
	"github.com/runfabric/runfabric/internal/state"
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
