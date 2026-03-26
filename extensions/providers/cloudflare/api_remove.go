package cloudflare

import (
	"context"
	"fmt"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remover deletes the Worker via Cloudflare API.
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	service := sdkprovider.Service(cfg)
	rv := sdkprovider.DecodeReceipt(receipt)
	if sdkprovider.Env("CLOUDFLARE_ACCOUNT_ID") == "" || sdkprovider.Env("CLOUDFLARE_API_TOKEN") == "" {
		return nil, fmt.Errorf("CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_API_TOKEN are required")
	}
	accountID := sdkprovider.Env("CLOUDFLARE_ACCOUNT_ID")
	name := rv.Metadata["worker"]
	if name == "" {
		name = fmt.Sprintf("%s-%s", service, stage)
	}
	url := cfAPI + "/accounts/" + accountID + "/workers/scripts/" + name
	if err := sdkprovider.DoDelete(ctx, url, "CLOUDFLARE_API_TOKEN"); err != nil {
		return nil, fmt.Errorf("cloudflare delete worker: %w", err)
	}
	return &sdkprovider.RemoveResult{Provider: "cloudflare-workers", Removed: true}, nil
}
