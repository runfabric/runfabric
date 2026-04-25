package digitalocean

import (
	"context"
	"fmt"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remover deletes the app via DigitalOcean API.
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	rv := sdkprovider.DecodeReceipt(receipt)
	appID := rv.Outputs["app_id"]
	if appID == "" {
		return nil, fmt.Errorf("receipt missing app_id; cannot remove DigitalOcean app")
	}
	url := doAPI + "/" + appID
	if err := sdkprovider.DoDelete(ctx, url, "DIGITALOCEAN_ACCESS_TOKEN"); err != nil {
		return nil, fmt.Errorf("digitalocean delete app: %w", err)
	}
	return &sdkprovider.RemoveResult{Provider: "digitalocean-functions", Removed: true}, nil
}
