package vercel

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remover deletes the project via Vercel API (DELETE /v9/projects/{name}).
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	coreCfg, err := sdkbridge.ToCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	_ = coreCfg
	rv := apiutil.DecodeReceipt(receipt)
	projectName := rv.Metadata["project"]
	if projectName == "" {
		projectName = coreCfg.Service
	}
	url := vercelAPI + "/v9/projects/" + projectName
	if err := apiutil.DoDelete(ctx, url, "VERCEL_TOKEN"); err != nil {
		return nil, fmt.Errorf("vercel delete project: %w", err)
	}
	return &sdkprovider.RemoveResult{Provider: "vercel", Removed: true}, nil
}
