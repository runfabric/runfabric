package netlify

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remover deletes the site via Netlify API (DELETE /sites/{id}).
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	coreCfg, err := sdkbridge.ToCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	_ = coreCfg
	rv := apiutil.DecodeReceipt(receipt)
	siteID := rv.Outputs["site_id"]
	if siteID == "" {
		siteID = rv.Metadata["site_id"]
	}
	if siteID == "" {
		return nil, fmt.Errorf("receipt missing site_id; cannot remove Netlify site")
	}
	url := netlifyAPI + "/sites/" + siteID
	if err := apiutil.DoDelete(ctx, url, "NETLIFY_AUTH_TOKEN"); err != nil {
		return nil, fmt.Errorf("netlify delete site: %w", err)
	}
	return &sdkprovider.RemoveResult{Provider: "netlify", Removed: true}, nil
}
