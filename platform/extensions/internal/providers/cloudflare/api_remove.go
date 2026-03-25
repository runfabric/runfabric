package cloudflare

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remover deletes the Worker via Cloudflare API.
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	coreCfg, err := sdkbridge.ToCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	rv := apiutil.DecodeReceipt(receipt)
	if apiutil.Env("CLOUDFLARE_ACCOUNT_ID") == "" || apiutil.Env("CLOUDFLARE_API_TOKEN") == "" {
		return nil, fmt.Errorf("CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_API_TOKEN are required")
	}
	accountID := apiutil.Env("CLOUDFLARE_ACCOUNT_ID")
	name := rv.Metadata["worker"]
	if name == "" {
		name = fmt.Sprintf("%s-%s", coreCfg.Service, stage)
	}
	url := cfAPI + "/accounts/" + accountID + "/workers/scripts/" + name
	if err := apiutil.DoDelete(ctx, url, "CLOUDFLARE_API_TOKEN"); err != nil {
		return nil, fmt.Errorf("cloudflare delete worker: %w", err)
	}
	return &sdkprovider.RemoveResult{Provider: "cloudflare-workers", Removed: true}, nil
}
