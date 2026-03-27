package netlify

import (
	"context"
	"fmt"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remover deletes the site via Netlify API (DELETE /sites/{id}).
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	rv := sdkprovider.DecodeReceipt(receipt)
	siteID := rv.Outputs["site_id"]
	if siteID == "" {
		siteID = rv.Metadata["site_id"]
	}
	if siteID == "" {
		return nil, fmt.Errorf("receipt missing site_id; cannot remove Netlify site")
	}
	url := netlifyAPI + "/sites/" + siteID
	if err := sdkprovider.DoDelete(ctx, url, "NETLIFY_AUTH_TOKEN"); err != nil {
		return nil, fmt.Errorf("netlify delete site: %w", err)
	}
	return &sdkprovider.RemoveResult{Provider: "netlify", Removed: true}, nil
}
