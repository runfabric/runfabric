package digitalocean

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remover deletes the app via DigitalOcean API.
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	coreCfg, err := sdkbridge.ToCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	_ = coreCfg
	rv := apiutil.DecodeReceipt(receipt)
	appID := rv.Outputs["app_id"]
	if appID == "" {
		return nil, fmt.Errorf("receipt missing app_id; cannot remove DigitalOcean app")
	}
	url := doAPI + "/" + appID
	if err := apiutil.DoDelete(ctx, url, "DIGITALOCEAN_ACCESS_TOKEN"); err != nil {
		return nil, fmt.Errorf("digitalocean delete app: %w", err)
	}
	return &sdkprovider.RemoveResult{Provider: "digitalocean-functions", Removed: true}, nil
}
