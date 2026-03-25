package gcp

import (
	"context"
	"fmt"
	"net/http"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remove loads the receipt and delegates to Remover.
func (p *Provider) Remove(ctx context.Context, req providers.RemoveRequest) (*providers.RemoveResult, error) {
	receipt, _ := state.Load(req.Root, req.Stage)
	sdkCfg, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := (Remover{}).Remove(ctx, sdkCfg, req.Stage, req.Root, receipt)
	if err != nil {
		return nil, err
	}
	return &providers.RemoveResult{Provider: r.Provider, Removed: r.Removed}, nil
}

// Remover deletes Cloud Functions via DELETE projects/.../locations/.../functions/...
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	coreCfg, err := sdkbridge.ToCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	rv := apiutil.DecodeReceipt(receipt)
	project := apiutil.Env("GCP_PROJECT")
	if project == "" {
		project = apiutil.Env("GCP_PROJECT_ID")
	}
	region := "us-central1"
	if rv.Outputs["region"] != "" {
		region = rv.Outputs["region"]
	}
	if project == "" || apiutil.Env("GCP_ACCESS_TOKEN") == "" {
		return &sdkprovider.RemoveResult{Provider: "gcp-functions", Removed: true}, nil
	}
	if coreCfg == nil {
		return &sdkprovider.RemoveResult{Provider: "gcp-functions", Removed: true}, nil
	}
	token := apiutil.Env("GCP_ACCESS_TOKEN")
	for fnName := range coreCfg.Functions {
		funcName := fmt.Sprintf("%s-%s-%s", coreCfg.Service, stage, fnName)
		url := fmt.Sprintf("%s/projects/%s/locations/%s/functions/%s", gcpAPI, project, region, funcName)
		req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		apiutil.DefaultClient.Do(req) // best effort
	}
	return &sdkprovider.RemoveResult{Provider: "gcp-functions", Removed: true}, nil
}
